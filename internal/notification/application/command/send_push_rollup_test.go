package command

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

type rollupMockNotifRepo struct {
	items []*aggregate.InAppNotification
}

var (
	_ repository.NotificationRepository   = (*rollupMockNotifRepo)(nil)
	_ repository.ReactionRollupRepository = (*rollupMockNotifRepo)(nil)
)

func (m *rollupMockNotifRepo) Save(_ context.Context, item *aggregate.InAppNotification) error {
	m.items = append(m.items, item)
	return nil
}

func (m *rollupMockNotifRepo) ListByUserID(_ context.Context, userID string, _, _ int32) ([]*aggregate.InAppNotification, int32, error) {
	return m.items, int32(len(m.items)), nil
}

func (m *rollupMockNotifRepo) MarkAsRead(_ context.Context, _, _ string) error { return nil }
func (m *rollupMockNotifRepo) MarkAllAsRead(_ context.Context, _ string) error { return nil }

func (m *rollupMockNotifRepo) FindUnreadReactionByPost(_ context.Context, userID, feedItemID string) (*aggregate.InAppNotification, error) {
	for i := len(m.items) - 1; i >= 0; i-- {
		it := m.items[i]
		if it.UserID() == userID && !it.IsRead() && it.Data()["type"] == "POST_REACTION" && it.Data()["feedItemId"] == feedItemID {
			return it, nil
		}
	}
	return nil, nil
}

func (m *rollupMockNotifRepo) UpdateNotificationContent(_ context.Context, item *aggregate.InAppNotification) error {
	for i := range m.items {
		if m.items[i].ID() == item.ID() {
			m.items[i] = item
			return nil
		}
	}
	return nil
}

type noopDeviceRepo struct{}

func (n *noopDeviceRepo) Save(_ context.Context, _ *aggregate.Device) error { return nil }
func (n *noopDeviceRepo) GetActiveDevicesByUserID(_ context.Context, _ string) ([]*aggregate.Device, error) {
	return nil, nil
}
func (n *noopDeviceRepo) DeactivateTokens(_ context.Context, _ []string) error { return nil }

func TestSendPushNotification_RollupsMultiplePostReactionsIntoSingleRow(t *testing.T) {
	repo := &rollupMockNotifRepo{}
	handler := NewSendPushNotificationHandler(&noopDeviceRepo{}, repo, nil, nil, nil, nil)

	// 1st reaction from Alice
	res1, err := handler.Handle(context.Background(), SendPushNotificationCommand{
		UserID: "owner-1",
		Title:  "Tương tác mới",
		Body:   "Alice đã bày tỏ cảm xúc về bài viết của bạn",
		Data: map[string]string{
			"type":       "POST_REACTION",
			"feedItemId": "post-99",
			"actorId":    "u-alice",
			"actorName":  "Alice",
		},
	})
	if err != nil {
		t.Fatalf("1st reaction failed: %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected 1 row after 1st reaction, got %d", len(repo.items))
	}

	// 2nd reaction from Bob on same post -> should roll up into same row!
	res2, err := handler.Handle(context.Background(), SendPushNotificationCommand{
		UserID: "owner-1",
		Title:  "Tương tác mới",
		Body:   "Bob đã bày tỏ cảm xúc về bài viết của bạn",
		Data: map[string]string{
			"type":       "POST_REACTION",
			"feedItemId": "post-99",
			"actorId":    "u-bob",
			"actorName":  "Bob",
		},
	})
	if err != nil {
		t.Fatalf("2nd reaction failed: %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected 1 rolled-up row after 2nd reaction, got %d", len(repo.items))
	}
	if res2.NotificationID != res1.NotificationID {
		t.Errorf("expected same notification ID %s, got %s", res1.NotificationID, res2.NotificationID)
	}
	if repo.items[0].Data()["rollupCount"] != "2" {
		t.Errorf("expected rollupCount=2, got %s", repo.items[0].Data()["rollupCount"])
	}
	wantBody := "Bob và 1 người khác đã bày tỏ cảm xúc về bài viết của bạn"
	if repo.items[0].Body() != wantBody {
		t.Errorf("expected body %q, got %q", wantBody, repo.items[0].Body())
	}
}
