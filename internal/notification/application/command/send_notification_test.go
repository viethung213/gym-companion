package command_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

type mockChannelDispatcher struct {
	channelName string
	dispatched  []command.NotificationMessage
	enrichMsg   *command.NotificationMessage
	err         error
}

func (m *mockChannelDispatcher) Channel() string {
	return m.channelName
}

func (m *mockChannelDispatcher) Dispatch(ctx context.Context, target port.NotificationTarget, msg command.NotificationMessage) (*command.NotificationMessage, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.channelName == "PUSH" && msg.InAppFailed {
		return nil, errors.New("push skipped: in-app persistence failed")
	}
	m.dispatched = append(m.dispatched, msg)
	if m.enrichMsg != nil {
		return m.enrichMsg, nil
	}
	return &msg, nil
}

func TestSendNotificationHandler_MultiChannelSuccess(t *testing.T) {
	ctx := context.Background()

	pushD := &mockChannelDispatcher{channelName: "PUSH"}
	inAppD := &mockChannelDispatcher{channelName: "IN_APP"}
	emailD := &mockChannelDispatcher{channelName: "EMAIL"}
	smsD := &mockChannelDispatcher{channelName: "SMS"}

	handler := command.NewSendNotificationHandler(pushD, inAppD, emailD, smsD)

	cmd := command.SendNotificationCommand{
		Target: port.NotificationTarget{
			Scope:  port.ScopeDirect,
			UserID: "user-123",
		},
		EventID:  "evt-999",
		Title:    "Workout Ready",
		Body:     "Your session is prepared",
		Channels: []string{"PUSH", "IN_APP", "EMAIL", "SMS"},
	}

	resp, err := handler.Handle(ctx, &cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Dispatched) != 4 {
		t.Errorf("expected 4 dispatched channels, got %d (%v)", len(resp.Dispatched), resp.Dispatched)
	}

	if len(pushD.dispatched) != 1 || len(inAppD.dispatched) != 1 || len(emailD.dispatched) != 1 || len(smsD.dispatched) != 1 {
		t.Errorf("expected each dispatcher to be called once")
	}
}

func TestSendNotificationHandler_DefaultChannels(t *testing.T) {
	ctx := context.Background()

	pushD := &mockChannelDispatcher{channelName: "PUSH"}
	inAppD := &mockChannelDispatcher{channelName: "IN_APP"}

	handler := command.NewSendNotificationHandler(pushD, inAppD)

	cmd := command.SendNotificationCommand{
		Target: port.NotificationTarget{
			Scope:  port.ScopeDirect,
			UserID: "user-123",
		},
		Title:    "Title",
		Body:     "Body",
		Channels: nil, // empty -> defaults to PUSH and IN_APP
	}

	resp, err := handler.Handle(ctx, &cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Dispatched) != 2 {
		t.Errorf("expected 2 default channels, got %d", len(resp.Dispatched))
	}
}

func TestSendNotificationHandler_PartialChannelFailure(t *testing.T) {
	ctx := context.Background()

	pushD := &mockChannelDispatcher{channelName: "PUSH"}
	emailD := &mockChannelDispatcher{channelName: "EMAIL", err: errors.New("smtp connection failed")}

	handler := command.NewSendNotificationHandler(pushD, emailD)

	cmd := command.SendNotificationCommand{
		Target: port.NotificationTarget{
			Scope:  port.ScopeDirect,
			UserID: "user-123",
		},
		Title:    "Title",
		Body:     "Body",
		Channels: []string{"PUSH", "EMAIL"},
	}

	resp, err := handler.Handle(ctx, &cmd)
	if err == nil {
		t.Fatalf("expected error from email failure, got nil")
	}

	// PUSH should have succeeded
	if len(resp.Dispatched) != 1 || resp.Dispatched[0] != "PUSH" {
		t.Errorf("expected PUSH to succeed despite EMAIL failure, got %v", resp.Dispatched)
	}
}

func TestPushChannelDispatcher_BroadcastTopic(t *testing.T) {
	ctx := context.Background()
	pushProv := &mockPushProvider{}
	deviceRepo := &mockDeviceRepo{}

	dispatcher := command.NewPushChannelDispatcher(deviceRepo, pushProv, nil, nil)

	// Broadcast topic to all_users
	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope: port.ScopeBroadcast,
	}, command.NotificationMessage{
		Title: "Broadcast",
		Body:  "To all users",
	})
	if err != nil {
		t.Fatalf("expected no error on broadcast push, got %v", err)
	}
}

func TestPushChannelDispatcher_GroupMulticastChunking(t *testing.T) {
	ctx := context.Background()
	pushProv := &mockPushProvider{resp: &port.PushResponse{
		SuccessCount:  500,
		InvalidTokens: []string{"t-u-1", "t-u-2"},
	}}
	deviceRepo := &mockDeviceRepo{}

	// Seed 600 active devices
	var userIDs []string
	for i := 1; i <= 600; i++ {
		uid := fmt.Sprintf("u-%d", i)
		userIDs = append(userIDs, uid)
		dev, _ := aggregate.NewDevice("d-"+uid, uid, "t-"+uid, vo.DeviceTypeIOS)
		_ = deviceRepo.Save(ctx, dev)
	}

	dispatcher := command.NewPushChannelDispatcher(deviceRepo, pushProv, nil, nil)

	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: userIDs,
	}, command.NotificationMessage{
		Title: "Multicast Title",
		Body:  "Multicast Body",
	})
	if err != nil {
		t.Fatalf("expected no error on multicast chunking, got %v", err)
	}
	// Called 2 times (500 + 100)
	if pushProv.callCount != 2 {
		t.Errorf("expected 2 push provider calls for 600 tokens (500 chunk), got %d", pushProv.callCount)
	}

	// Verify invalid tokens returned by FCM were deactivated in DB
	if len(deviceRepo.deactivatedTokens) == 0 {
		t.Errorf("expected invalid tokens from multicast to be deactivated, got 0")
	}
}

func TestInAppChannelDispatcher_BroadcastAndRollup(t *testing.T) {
	ctx := context.Background()
	notifRepo := &mockNotificationRepo{}

	dispatcher := command.NewInAppChannelDispatcher(notifRepo)

	// 1. Broadcast saves system announcement
	_, bErr := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope: port.ScopeBroadcast,
	}, command.NotificationMessage{
		EventID: "evt-b1",
		Title:   "System Maintenance",
		Body:    "Tonight at 2am",
	})
	if bErr != nil {
		t.Fatalf("expected no error on in-app broadcast announcement, got %v", bErr)
	}

	// 2. Group saves batch in-app
	_, gErr := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"u1", "u2", "u3"},
	}, command.NotificationMessage{
		EventID: "evt-g1",
		Title:   "Group Alert",
		Body:    "Cohort message",
	})
	if gErr != nil {
		t.Fatalf("expected no error on group batch in-app, got %v", gErr)
	}
	if len(notifRepo.items) != 3 {
		t.Errorf("expected 3 in-app records saved, got %d", len(notifRepo.items))
	}

	// 3. Direct saves single in-app
	_, dErr := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "u1",
	}, command.NotificationMessage{
		EventID: "evt-d1",
		Title:   "Personal Alert",
		Body:    "Your workout is ready",
	})
	if dErr != nil {
		t.Fatalf("expected no error on direct in-app, got %v", dErr)
	}
	if len(notifRepo.items) != 4 {
		t.Errorf("expected 4 in-app records saved, got %d", len(notifRepo.items))
	}
}

type mockEmailProv struct {
	sentEmails []struct {
		to      string
		subject string
		body    string
	}
}

func (m *mockEmailProv) SendEmail(ctx context.Context, to, subject, body string) error {
	m.sentEmails = append(m.sentEmails, struct {
		to      string
		subject string
		body    string
	}{to: to, subject: subject, body: body})
	return nil
}

func TestEmailChannelDispatcher_SuccessAndSuppression(t *testing.T) {
	ctx := context.Background()
	emailProv := &mockEmailProv{}
	settingRepo := &mockSettingRepoWithMap{settings: make(map[string]*aggregate.Setting)}

	dispatcher := command.NewEmailChannelDispatcher(emailProv, settingRepo)

	// 1. Success dispatch
	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "u1",
	}, command.NotificationMessage{
		Title: "Welcome",
		Body:  "Welcome to Gym Companion",
		Data:  map[string]string{"email": "u1@example.com"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(emailProv.sentEmails) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(emailProv.sentEmails))
	}

	// 2. Suppressed when enable_email is false
	s, _ := aggregate.NewDefaultSetting("u-disabled")
	_ = s.Update(true, false, true, "", "") // enable_email = false
	_ = settingRepo.Save(ctx, s)

	_, err2 := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "u-disabled",
	}, command.NotificationMessage{
		Title: "Marketing",
		Body:  "Buy now",
		Data:  map[string]string{"email": "disabled@example.com"},
	})
	if err2 != nil {
		t.Fatalf("expected no error on suppression, got %v", err2)
	}
	// Still 1 email (the new one was suppressed)
	if len(emailProv.sentEmails) != 1 {
		t.Errorf("expected email to be suppressed, but sent count is %d", len(emailProv.sentEmails))
	}

	// 3. Guard clause: ScopeGroup and ScopeBroadcast are safely skipped
	_, gErr := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"u1", "u2"},
	}, command.NotificationMessage{
		Title: "Group Email Attempt",
		Body:  "Should be skipped",
		Data:  map[string]string{"email": "group@example.com"},
	})
	if gErr != nil {
		t.Fatalf("expected no error on group email skip, got %v", gErr)
	}

	_, bErr := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope: port.ScopeBroadcast,
	}, command.NotificationMessage{
		Title: "Broadcast Email Attempt",
		Body:  "Should be skipped",
		Data:  map[string]string{"email": "all@example.com"},
	})
	if bErr != nil {
		t.Fatalf("expected no error on broadcast email skip, got %v", bErr)
	}

	// Email sent count should remain 1
	if len(emailProv.sentEmails) != 1 {
		t.Errorf("expected email count to stay 1 after group and broadcast skips, got %d", len(emailProv.sentEmails))
	}
}

type mockSMSProv struct {
	sentMessages []struct {
		phone   string
		message string
	}
}

func (m *mockSMSProv) SendSMS(ctx context.Context, phoneNumber, message string) error {
	m.sentMessages = append(m.sentMessages, struct {
		phone   string
		message string
	}{phone: phoneNumber, message: message})
	return nil
}

func TestSMSChannelDispatcher_Success(t *testing.T) {
	ctx := context.Background()
	smsProv := &mockSMSProv{}
	settingRepo := &mockSettingRepoWithMap{settings: make(map[string]*aggregate.Setting)}

	dispatcher := command.NewSMSChannelDispatcher(smsProv, settingRepo)

	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "u1",
	}, command.NotificationMessage{
		Body: "Your OTP is 123456",
		Data: map[string]string{"phone": "+84912345678"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(smsProv.sentMessages) != 1 {
		t.Fatalf("expected 1 sent SMS, got %d", len(smsProv.sentMessages))
	}
	if smsProv.sentMessages[0].phone != "+84912345678" {
		t.Errorf("expected phone +84912345678, got %s", smsProv.sentMessages[0].phone)
	}

	// Guard clause: ScopeGroup and ScopeBroadcast are safely skipped
	_, gErr := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"u1", "u2"},
	}, command.NotificationMessage{
		Body: "Group SMS should be skipped",
		Data: map[string]string{"phone": "+84912345678"},
	})
	if gErr != nil {
		t.Fatalf("expected no error on group SMS skip, got %v", gErr)
	}

	_, bErr := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope: port.ScopeBroadcast,
	}, command.NotificationMessage{
		Body: "Broadcast SMS should be skipped",
		Data: map[string]string{"phone": "+84912345678"},
	})
	if bErr != nil {
		t.Fatalf("expected no error on broadcast SMS skip, got %v", bErr)
	}

	if len(smsProv.sentMessages) != 1 {
		t.Errorf("expected SMS count to stay 1 after group and broadcast skips, got %d", len(smsProv.sentMessages))
	}
}

func TestSendNotificationHandler_Pipeline_EnrichmentInRAM(t *testing.T) {
	ctx := context.Background()

	// IN_APP modifies the message in RAM (e.g. rollup)
	inAppEnriched := &command.NotificationMessage{
		EventID: "evt-feed-1",
		Title:   "Tương tác mới",
		Body:    "Alice và 3 người khác đã bày tỏ cảm xúc về bài viết của bạn",
		Data: map[string]string{
			"type":        "POST_REACTION",
			"feedItemId":  "post-100",
			"rollupCount": "4",
		},
	}
	inAppD := &mockChannelDispatcher{
		channelName: "IN_APP",
		enrichMsg:   inAppEnriched,
	}

	pushD := &mockChannelDispatcher{channelName: "PUSH"}

	// Handlers registered in order: IN_APP then PUSH
	handler := command.NewSendNotificationHandler(inAppD, pushD)

	cmd := command.SendNotificationCommand{
		Target: port.NotificationTarget{
			Scope:  port.ScopeDirect,
			UserID: "owner-1",
		},
		EventID:  "evt-feed-1",
		Title:    "Tương tác mới",
		Body:     "Alice đã bày tỏ cảm xúc",
		Channels: []string{"IN_APP", "PUSH"},
	}

	resp, err := handler.Handle(ctx, &cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Dispatched) != 2 {
		t.Fatalf("expected 2 dispatched channels, got %d", len(resp.Dispatched))
	}

	// Verify that PUSH received the ENRICHED message from IN_APP in RAM!
	if len(pushD.dispatched) != 1 {
		t.Fatalf("expected 1 push dispatch, got %d", len(pushD.dispatched))
	}
	receivedPush := pushD.dispatched[0]
	if receivedPush.Body != "Alice và 3 người khác đã bày tỏ cảm xúc về bài viết của bạn" {
		t.Errorf("expected rolled up body in PUSH, got: %s", receivedPush.Body)
	}
	if receivedPush.Data["rollupCount"] != "4" {
		t.Errorf("expected rollupCount 4 in PUSH, got: %s", receivedPush.Data["rollupCount"])
	}
}

type mockCooldownService struct {
	allowed      bool
	allowedUsers map[string]bool
	releasedKeys []string
}

func (m *mockCooldownService) CheckAndLock(_ context.Context, feedID, userID string, _ time.Duration) (bool, error) {
	if m.allowedUsers != nil {
		return m.allowedUsers[userID], nil
	}
	return m.allowed, nil
}

func (m *mockCooldownService) CheckAndLockBatch(_ context.Context, feedID string, recipientIDs []string, _ time.Duration) ([]string, error) {
	var allowed []string
	for _, id := range recipientIDs {
		isAllowed := m.allowed
		if m.allowedUsers != nil {
			isAllowed = m.allowedUsers[id]
		}
		if isAllowed {
			allowed = append(allowed, id)
		}
	}
	return allowed, nil
}

func (m *mockCooldownService) ReleaseLock(_ context.Context, feedID, userID string) error {
	m.releasedKeys = append(m.releasedKeys, feedID+":"+userID)
	return nil
}

func (m *mockCooldownService) ReleaseLockBatch(_ context.Context, feedID string, recipientIDs []string) error {
	for _, id := range recipientIDs {
		m.releasedKeys = append(m.releasedKeys, feedID+":"+id)
	}
	return nil
}

func TestPushChannelDispatcher_CooldownAndSelfHealing(t *testing.T) {
	ctx := context.Background()

	// 1. Test Cooldown Suppressed
	pushProv := &mockPushProvider{}
	deviceRepo := &mockDeviceRepo{}
	dev, _ := aggregate.NewDevice("d-1", "user-1", "tok-1", vo.DeviceTypeAndroid)
	_ = deviceRepo.Save(ctx, dev)

	cooldownSvc := &mockCooldownService{allowed: false} // Cooldown active

	dispatcher := command.NewPushChannelDispatcher(deviceRepo, pushProv, nil, cooldownSvc)

	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Title: "Comment",
		Body:  "Bob commented",
		Data:  map[string]string{"feedItemId": "post-99"},
	})
	if err != nil {
		t.Fatalf("expected no error on suppressed push, got %v", err)
	}
	// Push provider should NOT have been called because cooldown was active
	if pushProv.callCount != 0 {
		t.Errorf("expected FCM not to be called when cooldown is active, got %d calls", pushProv.callCount)
	}

	// 2. Test Self-Healing when FCM call fails
	failingPushProv := &mockPushProvider{err: errors.New("fcm network timeout")}
	cooldownSvc2 := &mockCooldownService{allowed: true}

	dispatcher2 := command.NewPushChannelDispatcher(deviceRepo, failingPushProv, nil, cooldownSvc2)

	_, err2 := dispatcher2.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, command.NotificationMessage{
		Title: "Comment",
		Body:  "Bob commented",
		Data:  map[string]string{"feedItemId": "post-99"},
	})
	if err2 != nil {
		t.Fatalf("expected best-effort error swallowing for push provider failure, got %v", err2)
	}

	// Verify that ReleaseLock was called (Self-healing)
	if len(cooldownSvc2.releasedKeys) != 1 || cooldownSvc2.releasedKeys[0] != "post-99:user-1" {
		t.Errorf("expected self-healing ReleaseLock to be called for post-99:user-1, got %v", cooldownSvc2.releasedKeys)
	}
}

func TestSendNotificationHandler_InAppFailureSkipsPushButDispatchesSMSAndEmail(t *testing.T) {
	ctx := context.Background()

	// IN_APP fails with DB error
	inAppD := &mockChannelDispatcher{
		channelName: "IN_APP",
		err:         errors.New("db disk full"),
	}
	pushD := &mockChannelDispatcher{channelName: "PUSH"}
	smsD := &mockChannelDispatcher{channelName: "SMS"}
	emailD := &mockChannelDispatcher{channelName: "EMAIL"}

	handler := command.NewSendNotificationHandler(inAppD, pushD, smsD, emailD)

	cmd := command.SendNotificationCommand{
		Target: port.NotificationTarget{
			Scope:  port.ScopeDirect,
			UserID: "u1",
		},
		EventID:  "evt-otp-1",
		Title:    "Mã xác thực OTP",
		Body:     "Mã OTP của bạn là 888999",
		Channels: []string{"IN_APP", "PUSH", "SMS", "EMAIL"},
	}

	resp, err := handler.Handle(ctx, &cmd)
	if err == nil {
		t.Fatalf("expected error from IN_APP failure, got nil")
	}

	// 1. PUSH must have been SKIPPED (not dispatched)
	if len(pushD.dispatched) != 0 {
		t.Errorf("expected PUSH to be skipped when IN_APP fails, got %d calls", len(pushD.dispatched))
	}

	// 2. SMS and EMAIL must have been DISPATCHED successfully
	if len(smsD.dispatched) != 1 {
		t.Errorf("expected SMS to be dispatched despite IN_APP failure, got %d calls", len(smsD.dispatched))
	}
	if len(emailD.dispatched) != 1 {
		t.Errorf("expected EMAIL to be dispatched despite IN_APP failure, got %d calls", len(emailD.dispatched))
	}

	// 3. Response should record SMS and EMAIL as successfully dispatched (order may vary due to concurrent fan-out)
	if len(resp.Dispatched) != 2 {
		t.Fatalf("expected 2 dispatched channels, got %d (%v)", len(resp.Dispatched), resp.Dispatched)
	}
	hasSMS := resp.Dispatched[0] == "SMS" || resp.Dispatched[1] == "SMS"
	hasEmail := resp.Dispatched[0] == "EMAIL" || resp.Dispatched[1] == "EMAIL"
	if !hasSMS || !hasEmail {
		t.Errorf("expected Dispatched to contain SMS and EMAIL, got %v", resp.Dispatched)
	}
}

func TestPushChannelDispatcher_InAppFailedSkipsDelivery(t *testing.T) {
	ctx := context.Background()
	pushProv := &mockPushProvider{}
	deviceRepo := &mockDeviceRepo{}
	dev, _ := aggregate.NewDevice("d-1", "u-1", "tok-1", vo.DeviceTypeAndroid)
	_ = deviceRepo.Save(ctx, dev)

	dispatcher := command.NewPushChannelDispatcher(deviceRepo, pushProv, nil, nil)

	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "u-1",
	}, command.NotificationMessage{
		Title:       "Test",
		Body:        "Test",
		InAppFailed: true, // Passed in RAM from IN_APP failure
	})

	if err == nil {
		t.Fatalf("expected error indicating push skipped, got nil")
	}
	if pushProv.callCount != 0 {
		t.Errorf("expected FCM push provider not to be called when InAppFailed is true, got %d", pushProv.callCount)
	}
}

func TestPushChannelDispatcher_BroadcastCooldown(t *testing.T) {
	ctx := context.Background()
	pushProv := &mockPushProvider{}
	cooldownSvc := &mockCooldownService{allowed: false} // Broadcast cooldown active

	dispatcher := command.NewPushChannelDispatcher(nil, pushProv, nil, cooldownSvc)

	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope: port.ScopeBroadcast,
	}, command.NotificationMessage{
		Title: "System Announcement",
		Body:  "Broadcast Body",
		Data:  map[string]string{"feedItemId": "feed-broadcast-1"},
	})
	if err != nil {
		t.Fatalf("expected no error on suppressed broadcast push, got %v", err)
	}
	if pushProv.callCount != 0 {
		t.Errorf("expected 0 broadcast calls when cooldown active, got %d", pushProv.callCount)
	}
}

func TestPushChannelDispatcher_GroupCooldownFiltering(t *testing.T) {
	ctx := context.Background()
	pushProv := &mockPushProvider{resp: &port.PushResponse{SuccessCount: 1}}
	deviceRepo := &mockDeviceRepo{}
	dev1, _ := aggregate.NewDevice("d-1", "user-cooldown", "tok-1", vo.DeviceTypeAndroid)
	dev2, _ := aggregate.NewDevice("d-2", "user-allowed", "tok-2", vo.DeviceTypeAndroid)
	_ = deviceRepo.Save(ctx, dev1)
	_ = deviceRepo.Save(ctx, dev2)

	// user-cooldown is suppressed (false), user-allowed is permitted (true)
	cooldownSvc := &mockCooldownService{
		allowedUsers: map[string]bool{
			"user-cooldown": false,
			"user-allowed":  true,
		},
	}

	dispatcher := command.NewPushChannelDispatcher(deviceRepo, pushProv, nil, cooldownSvc)

	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"user-cooldown", "user-allowed"},
	}, command.NotificationMessage{
		Title: "Group Comment",
		Body:  "New comment in group",
		Data:  map[string]string{"feedItemId": "feed-grp-1"},
	})
	if err != nil {
		t.Fatalf("expected no error on group cooldown filtering, got %v", err)
	}

	// Verify push provider was called for the allowed user
	if pushProv.callCount != 1 {
		t.Fatalf("expected 1 call to push provider for allowed group user, got %d", pushProv.callCount)
	}
}

type mockSettingRepoForNotification struct {
	settings map[string]*aggregate.Setting
}

func (m *mockSettingRepoForNotification) GetByUserID(_ context.Context, userID string) (*aggregate.Setting, error) {
	if s, exists := m.settings[userID]; exists {
		return s, nil
	}
	return nil, derror.ErrSettingNotFound
}

func (m *mockSettingRepoForNotification) GetByUserIDs(_ context.Context, userIDs []string) ([]*aggregate.Setting, error) {
	var list []*aggregate.Setting
	for _, uid := range userIDs {
		if s, exists := m.settings[uid]; exists {
			list = append(list, s)
		}
	}
	return list, nil
}

func (m *mockSettingRepoForNotification) Save(_ context.Context, s *aggregate.Setting) error {
	m.settings[s.UserID()] = s
	return nil
}

func TestPushChannelDispatcher_GroupPrivacySettings(t *testing.T) {
	ctx := context.Background()
	pushProv := &mockPushProvider{resp: &port.PushResponse{SuccessCount: 1}}
	deviceRepo := &mockDeviceRepo{}
	dev1, _ := aggregate.NewDevice("d-1", "user-disabled", "tok-1", vo.DeviceTypeAndroid)
	dev2, _ := aggregate.NewDevice("d-2", "user-allowed", "tok-2", vo.DeviceTypeAndroid)
	_ = deviceRepo.Save(ctx, dev1)
	_ = deviceRepo.Save(ctx, dev2)

	disabledSetting := aggregate.ReconstituteSetting("user-disabled", false, true, true, "", "", time.Now(), time.Now())
	allowedSetting := aggregate.ReconstituteSetting("user-allowed", true, true, true, "", "", time.Now(), time.Now())

	settingRepo := &mockSettingRepoForNotification{
		settings: map[string]*aggregate.Setting{
			"user-disabled": disabledSetting,
			"user-allowed":  allowedSetting,
		},
	}

	dispatcher := command.NewPushChannelDispatcher(deviceRepo, pushProv, settingRepo, nil)

	// 1. Normal priority group push: user-disabled must be filtered out by privacy setting
	_, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"user-disabled", "user-allowed"},
	}, command.NotificationMessage{
		Title: "Group Workout",
		Body:  "Workout session ready",
	})
	if err != nil {
		t.Fatalf("expected no error on group push, got %v", err)
	}
	if pushProv.callCount != 1 {
		t.Fatalf("expected 1 push call for allowed user, got %d", pushProv.callCount)
	}

	// 2. High priority group push: bypasses privacy settings, delivers to all active devices
	pushProv.callCount = 0
	_, err = dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"user-disabled", "user-allowed"},
	}, command.NotificationMessage{
		Title:          "Urgent Alert",
		Body:           "Gym fire alarm evacuation",
		IsHighPriority: true,
	})
	if err != nil {
		t.Fatalf("expected no error on high priority group push, got %v", err)
	}
	if pushProv.callCount != 1 {
		t.Fatalf("expected push call for high priority, got %d", pushProv.callCount)
	}
}
