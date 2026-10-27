package consumer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/consumer"
)

type mockUserSnapshotRepo struct {
	snapshots map[string]*entity.UserSnapshot
	err       error
}

func (m *mockUserSnapshotRepo) Upsert(ctx context.Context, snap *entity.UserSnapshot) error {
	if m.err != nil {
		return m.err
	}
	m.snapshots[snap.UserID()] = snap
	return nil
}

func (m *mockUserSnapshotRepo) FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return m.snapshots[userID], nil
}

func (m *mockUserSnapshotRepo) FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) {
	return nil, nil
}

type mockOutboxLogRepo struct {
	processed map[string]bool
	saved     map[string]string // eventID -> status
}

func (m *mockOutboxLogRepo) Save(ctx context.Context, eventID, eventType string, payload []byte, partitionKey, status, errMsg string) error {
	m.saved[eventID] = status
	if status == "PROCESSED" || status == "SUCCESS" {
		m.processed[eventID] = true
	}
	return nil
}

func (m *mockOutboxLogRepo) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	return m.processed[eventID], nil
}

func TestUserIdentityUpdatedConsumer(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when payload is invalid json", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		c := consumer.NewUserIdentityUpdatedConsumer(handler, nil)

		err := c.HandleMessage(ctx, []byte(`invalid json`))
		if err == nil {
			t.Errorf("expected error for invalid json, got nil")
		}
	})

	t.Run("ignores events that are not UserIdentityUpdated", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		c := consumer.NewUserIdentityUpdatedConsumer(handler, nil)

		payload := []byte(`{
			"id": "evt_1",
			"type": "contracts.core.workout.v1.event.WorkoutCompleted",
			"data": {}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil for ignored event type, got: %v", err)
		}
		if len(repo.snapshots) != 0 {
			t.Errorf("expected no snapshots to be updated")
		}
	})

	t.Run("skips already processed event (idempotency)", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: map[string]bool{"evt_already_done": true},
			saved:     make(map[string]string),
		}
		c := consumer.NewUserIdentityUpdatedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_already_done",
			"type": "contracts.supporting.profile.v1.event.UserIdentityUpdated",
			"data": {
				"userId": "u_done",
				"fullName": "Alice",
				"avatarUrl": "https://avatar.png",
				"updatedAt": "2026-10-18T10:00:00Z"
			}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil for already processed event: %v", err)
		}
		if _, ok := repo.snapshots["u_done"]; ok {
			t.Errorf("handler should not have been called for idempotent event")
		}
	})

	t.Run("successfully processes UserIdentityUpdated event and logs as PROCESSED", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewUserIdentityUpdatedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_new_profile",
			"type": "contracts.supporting.profile.v1.event.UserIdentityUpdated",
			"data": {
				"userId": "u_fresh",
				"fullName": "Nguyen Van B",
				"avatarUrl": "https://avatar.png/b.png",
				"updatedAt": "2026-10-18T12:00:00Z"
			}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("HandleMessage error: %v", err)
		}

		snap, ok := repo.snapshots["u_fresh"]
		if !ok || snap == nil {
			t.Fatalf("expected snapshot to be saved")
		}
		if snap.DisplayName() != "Nguyen Van B" {
			t.Errorf("got name %s, want Nguyen Van B", snap.DisplayName())
		}
		if snap.AvatarURL() != "https://avatar.png/b.png" {
			t.Errorf("got avatar %s, want https://avatar.png/b.png", snap.AvatarURL())
		}
		expectedTime, _ := time.Parse(time.RFC3339, "2026-10-18T12:00:00Z")
		if !snap.SyncedAt().Equal(expectedTime) {
			t.Errorf("got synced_at %v, want %v", snap.SyncedAt(), expectedTime)
		}
		if logRepo.saved["evt_new_profile"] != "PROCESSED" {
			t.Errorf("expected outbox log status PROCESSED, got %s", logRepo.saved["evt_new_profile"])
		}
	})

	t.Run("logs as FAILED and returns error when handler fails", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{
			snapshots: make(map[string]*entity.UserSnapshot),
			err:       errors.New("db error"),
		}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewUserIdentityUpdatedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_err",
			"type": "contracts.supporting.profile.v1.event.UserIdentityUpdated",
			"data": {
				"userId": "u_err",
				"fullName": "Error User"
			}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err == nil {
			t.Fatalf("expected error from handler, got nil")
		}
		if logRepo.saved["evt_err"] != "FAILED" {
			t.Errorf("expected outbox log status FAILED, got %s", logRepo.saved["evt_err"])
		}
	})
}
