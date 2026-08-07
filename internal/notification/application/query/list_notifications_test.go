package query_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/query"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
)

type mockNotificationRepo struct {
	items []*aggregate.InAppNotification
}

func (m *mockNotificationRepo) Save(ctx context.Context, item *aggregate.InAppNotification) error {
	m.items = append(m.items, item)
	return nil
}

func (m *mockNotificationRepo) ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error) {
	return m.items, int32(len(m.items)), nil
}

func (m *mockNotificationRepo) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	return nil
}

func (m *mockNotificationRepo) MarkAllAsRead(ctx context.Context, userID string) error {
	return nil
}

func TestListNotificationsHandler(t *testing.T) {
	t.Parallel()

	repo := &mockNotificationRepo{}
	item, _ := aggregate.NewInAppNotification("notif-1", "usr-1", "Title", "Body", nil)
	_ = repo.Save(context.Background(), item)

	handler := query.NewListNotificationsHandler(repo)

	t.Run("list notifications success", func(t *testing.T) {
		t.Parallel()

		res, err := handler.Handle(context.Background(), query.ListNotificationsQuery{
			UserID: "usr-1",
			Limit:  10,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if got, want := res.TotalCount, int32(1); got != want {
			t.Errorf("got TotalCount %d, want %d", got, want)
		}
		if got, want := len(res.Items), 1; got != want {
			t.Errorf("got Items len %d, want %d", got, want)
		}
	})
}
