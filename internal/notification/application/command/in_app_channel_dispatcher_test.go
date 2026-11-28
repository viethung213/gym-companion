package command_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

type mockInAppRepo struct {
	items         []*aggregate.InAppNotification
	announcements []*aggregate.SystemAnnouncement
}

var (
	_ repository.NotificationRepository   = (*mockInAppRepo)(nil)
	_ repository.ReactionRollupRepository = (*mockInAppRepo)(nil)
)

func (m *mockInAppRepo) Save(_ context.Context, item *aggregate.InAppNotification) error {
	m.items = append(m.items, item)
	return nil
}

func (m *mockInAppRepo) BatchSave(_ context.Context, items []*aggregate.InAppNotification) error {
	m.items = append(m.items, items...)
	return nil
}

func (m *mockInAppRepo) SaveSystemAnnouncement(_ context.Context, announcement *aggregate.SystemAnnouncement) error {
	m.announcements = append(m.announcements, announcement)
	return nil
}

func (m *mockInAppRepo) MarkAnnouncementAsRead(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockInAppRepo) ListByUserID(_ context.Context, userID string, _, _ int32) ([]*aggregate.InAppNotification, int32, error) {
	var userItems []*aggregate.InAppNotification
	for _, it := range m.items {
		if it.UserID() == userID {
			userItems = append(userItems, it)
		}
	}
	return userItems, int32(len(userItems)), nil
}

func (m *mockInAppRepo) MarkAsRead(_ context.Context, userID, notificationID string) error {
	for _, it := range m.items {
		if it.UserID() == userID && it.ID() == notificationID {
			it.MarkAsRead()
			return nil
		}
	}
	return nil
}

func (m *mockInAppRepo) MarkAllAsRead(_ context.Context, userID string) error {
	for _, it := range m.items {
		if it.UserID() == userID {
			it.MarkAsRead()
		}
	}
	return nil
}

func (m *mockInAppRepo) FindByUserAndEventID(_ context.Context, userID, eventID string) (*aggregate.InAppNotification, error) {
	for _, it := range m.items {
		if it.UserID() == userID && it.EventID() == eventID {
			return it, nil
		}
	}
	return nil, nil
}

func (m *mockInAppRepo) FindUnreadReactionByPost(_ context.Context, userID, feedItemID string) (*aggregate.InAppNotification, error) {
	for i := len(m.items) - 1; i >= 0; i-- {
		it := m.items[i]
		if it.UserID() == userID && !it.IsRead() && it.Data()["type"] == "POST_REACTION" && it.Data()["feedItemId"] == feedItemID {
			return it, nil
		}
	}
	return nil, nil
}

func (m *mockInAppRepo) FindUnreadReactionsByPostAndUsers(_ context.Context, userIDs []string, feedItemID string) ([]*aggregate.InAppNotification, error) {
	userSet := make(map[string]bool, len(userIDs))
	for _, uid := range userIDs {
		userSet[uid] = true
	}
	var res []*aggregate.InAppNotification
	for i := len(m.items) - 1; i >= 0; i-- {
		it := m.items[i]
		if userSet[it.UserID()] && !it.IsRead() && it.Data()["type"] == "POST_REACTION" && it.Data()["feedItemId"] == feedItemID {
			res = append(res, it)
		}
	}
	return res, nil
}

func (m *mockInAppRepo) UpdateNotificationContent(_ context.Context, item *aggregate.InAppNotification) error {
	for i := range m.items {
		if m.items[i].ID() == item.ID() {
			m.items[i] = item
			return nil
		}
	}
	return nil
}

func (m *mockInAppRepo) BatchUpdateNotificationContent(_ context.Context, items []*aggregate.InAppNotification) error {
	for _, it := range items {
		for i := range m.items {
			if m.items[i].ID() == it.ID() {
				m.items[i] = it
				break
			}
		}
	}
	return nil
}

func TestInAppChannelDispatcher_ScopeBroadcast(t *testing.T) {
	ctx := context.Background()
	repo := &mockInAppRepo{}
	dispatcher := command.NewInAppChannelDispatcher(repo)

	if dispatcher.Channel() != "IN_APP" {
		t.Fatalf("expected channel IN_APP, got %s", dispatcher.Channel())
	}

	msg := command.NotificationMessage{
		EventID: "evt-broadcast-1",
		Title:   "System Maintenance",
		Body:    "System will be offline at 2am",
		Data:    map[string]string{"type": "MAINTENANCE"},
	}

	res, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope: port.ScopeBroadcast,
	}, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Title != msg.Title {
		t.Fatalf("expected response message to match input, got %v", res)
	}

	if len(repo.announcements) != 1 {
		t.Fatalf("expected 1 announcement saved, got %d", len(repo.announcements))
	}
	if repo.announcements[0].Title() != msg.Title {
		t.Errorf("expected announcement title %s, got %s", msg.Title, repo.announcements[0].Title())
	}
}

func TestInAppChannelDispatcher_ScopeDirect_NonSocial(t *testing.T) {
	ctx := context.Background()
	repo := &mockInAppRepo{}
	dispatcher := command.NewInAppChannelDispatcher(repo)

	msg := command.NotificationMessage{
		EventID: "evt-meal-1",
		Title:   "Meal Reminder",
		Body:    "Time for lunch",
		Data:    map[string]string{"type": "MEAL_REMINDER"},
	}

	res, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-1",
	}, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Title != msg.Title {
		t.Fatalf("expected message to return unchanged")
	}

	if len(repo.items) != 1 {
		t.Fatalf("expected 1 item saved, got %d", len(repo.items))
	}
	if repo.items[0].UserID() != "user-1" {
		t.Errorf("expected user-1, got %s", repo.items[0].UserID())
	}
}

func TestInAppChannelDispatcher_ScopeDirect_SocialRollupAndReadStatus(t *testing.T) {
	ctx := context.Background()
	repo := &mockInAppRepo{}
	dispatcher := command.NewInAppChannelDispatcher(repo)

	// 1. Initial reaction message dispatch (creates first notification)
	msg1 := command.NotificationMessage{
		EventID: "evt-react-1",
		Title:   "New Reaction",
		Body:    "Alice đã bày tỏ cảm xúc",
		Data: map[string]string{
			"type":       "POST_REACTION",
			"feedItemId": "feed-100",
			"actorName":  "Alice",
		},
	}

	res1, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-target",
	}, msg1)
	if err != nil {
		t.Fatalf("first dispatch failed: %v", err)
	}
	if res1 == nil {
		t.Fatalf("expected non-nil response")
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected 1 item in repo, got %d", len(repo.items))
	}
	if repo.items[0].IsRead() {
		t.Fatalf("expected initial item to be unread")
	}

	// 2. Second reaction on same post while UNREAD -> Rollup occurs!
	msg2 := command.NotificationMessage{
		EventID: "evt-react-2",
		Title:   "New Reaction",
		Body:    "Bob đã bày tỏ cảm xúc",
		Data: map[string]string{
			"type":       "POST_REACTION",
			"feedItemId": "feed-100",
			"actorName":  "Bob",
		},
	}

	res2, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-target",
	}, msg2)
	if err != nil {
		t.Fatalf("second dispatch failed: %v", err)
	}
	if res2 == nil {
		t.Fatalf("expected non-nil response for rolled up message")
	}

	// Repo item count should still be 1 (updated in place)
	if len(repo.items) != 1 {
		t.Fatalf("expected still 1 item after rollup, got %d", len(repo.items))
	}
	if repo.items[0].Data()["rollupCount"] != "2" {
		t.Errorf("expected rollupCount 2, got %s", repo.items[0].Data()["rollupCount"])
	}
	expectedBody := "Bob và 1 người khác đã bày tỏ cảm xúc về bài viết của bạn"
	if res2.Body != expectedBody {
		t.Errorf("expected rolled body %q, got %q", expectedBody, res2.Body)
	}

	// 3. User reads the notification
	repo.items[0].MarkAsRead()
	if !repo.items[0].IsRead() {
		t.Fatalf("expected notification to be marked as read")
	}

	// 4. Third reaction on same post after READ -> Inserts NEW row!
	msg3 := command.NotificationMessage{
		EventID: "evt-react-3",
		Title:   "New Reaction",
		Body:    "Charlie đã bày tỏ cảm xúc",
		Data: map[string]string{
			"type":       "POST_REACTION",
			"feedItemId": "feed-100",
			"actorName":  "Charlie",
		},
	}

	res3, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: "user-target",
	}, msg3)
	if err != nil {
		t.Fatalf("third dispatch failed: %v", err)
	}
	if res3 == nil {
		t.Fatalf("expected non-nil response")
	}

	// Repo item count should now be 2!
	if len(repo.items) != 2 {
		t.Fatalf("expected 2 items after new reaction following read, got %d", len(repo.items))
	}
}

func TestInAppChannelDispatcher_ScopeGroup_NonSocial(t *testing.T) {
	ctx := context.Background()
	repo := &mockInAppRepo{}
	dispatcher := command.NewInAppChannelDispatcher(repo)

	// Group notification for gym class schedule (non-social)
	msg := command.NotificationMessage{
		EventID: "evt-group-workout",
		Title:   "Class Starting",
		Body:    "CrossFit session starting in 15 mins",
		Data: map[string]string{
			"type": "WORKOUT_REMINDER",
		},
	}

	// Include duplicate user-1 to verify deduplication
	res, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"user-1", "user-2", "user-3", "user-1"},
	}, msg)
	if err != nil {
		t.Fatalf("group non-social dispatch failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil response")
	}

	// Expect exactly 3 distinct items saved via BatchSave
	if len(repo.items) != 3 {
		t.Fatalf("expected 3 items saved for group, got %d", len(repo.items))
	}
}

func TestInAppChannelDispatcher_ScopeGroup_SocialRollup(t *testing.T) {
	ctx := context.Background()
	repo := &mockInAppRepo{}
	dispatcher := command.NewInAppChannelDispatcher(repo)

	// Pre-seed user-a with an UNREAD reaction on post-200
	itemA, _ := aggregate.NewInAppNotification(
		"notif-a",
		"user-a",
		"Reaction",
		"Alice đã bày tỏ cảm xúc",
		map[string]string{
			"type":        "POST_REACTION",
			"feedItemId":  "post-200",
			"actorName":   "Alice",
			"rollupCount": "1",
		},
	)
	_ = repo.Save(ctx, itemA)

	// Pre-seed user-b with an ALREADY READ reaction on post-200
	itemB, _ := aggregate.NewInAppNotification(
		"notif-b",
		"user-b",
		"Reaction",
		"Alice đã bày tỏ cảm xúc",
		map[string]string{
			"type":        "POST_REACTION",
			"feedItemId":  "post-200",
			"actorName":   "Alice",
			"rollupCount": "1",
		},
	)
	itemB.MarkAsRead()
	_ = repo.Save(ctx, itemB)

	// user-c has no prior notifications at all

	// Dispatch group reaction notification to user-a, user-b, user-c
	groupMsg := command.NotificationMessage{
		EventID: "evt-grp-react",
		Title:   "New Reaction",
		Body:    "David đã bày tỏ cảm xúc",
		Data: map[string]string{
			"type":       "POST_REACTION",
			"feedItemId": "post-200",
			"actorName":  "David",
		},
	}

	res, err := dispatcher.Dispatch(ctx, port.NotificationTarget{
		Scope:   port.ScopeGroup,
		UserIDs: []string{"user-a", "user-b", "user-c"},
	}, groupMsg)
	if err != nil {
		t.Fatalf("group social dispatch failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil response message")
	}

	// 1. user-a had an UNREAD notification -> rolled up in place! (count becomes 2)
	updatedA, _ := repo.FindUnreadReactionByPost(ctx, "user-a", "post-200")
	if updatedA == nil {
		t.Fatalf("expected user-a unread notification to exist")
	}
	if updatedA.Data()["rollupCount"] != "2" {
		t.Errorf("expected user-a rollupCount 2, got %s", updatedA.Data()["rollupCount"])
	}

	// 2. user-b already read their previous reaction -> a NEW notification was inserted!
	userBItems, countB, _ := repo.ListByUserID(ctx, "user-b", 10, 0)
	if countB != 2 || len(userBItems) != 2 {
		t.Fatalf("expected 2 notifications for user-b (1 read + 1 new), got %d", countB)
	}

	// 3. user-c had no prior notification -> 1 new notification created!
	userCItems, countC, _ := repo.ListByUserID(ctx, "user-c", 10, 0)
	if countC != 1 || len(userCItems) != 1 {
		t.Fatalf("expected 1 notification for user-c, got %d", countC)
	}

	// 4. Returned effective message for ScopeGroup: preserves uniform original event message for multicast push
	if res.Body != groupMsg.Body {
		t.Errorf("expected returned body %q, got %q", groupMsg.Body, res.Body)
	}
}
