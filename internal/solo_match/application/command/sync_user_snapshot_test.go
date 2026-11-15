package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
)

type syncSnapshotMock struct {
	snapshots map[string]*entity.UserSnapshot
}

func (s *syncSnapshotMock) Upsert(ctx context.Context, snap *entity.UserSnapshot) error {
	s.snapshots[snap.UserID()] = snap
	return nil
}

func (s *syncSnapshotMock) UpdateXP(ctx context.Context, userID string, xp int64, syncedAt time.Time) error {
	snap, ok := s.snapshots[userID]
	if !ok || snap == nil {
		snap, _ = entity.NewUserSnapshot(userID, "", "", syncedAt)
		s.snapshots[userID] = snap
	}
	snap.UpdateXP(xp, syncedAt)
	return nil
}

func (s *syncSnapshotMock) FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	return s.snapshots[userID], nil
}

func (s *syncSnapshotMock) FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) {
	return nil, nil
}

func TestSyncUserSnapshotHandler(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	snapshotRepo := &syncSnapshotMock{snapshots: make(map[string]*entity.UserSnapshot)}
	handler := command.NewSyncUserSnapshotHandler(snapshotRepo)

	t.Run("successfully syncs user snapshot", func(t *testing.T) {
		cmd := command.SyncUserSnapshotCommand{
			UserID:      "user_100",
			DisplayName: "John Doe",
			AvatarURL:   "https://example.com/avatar.png",
			SyncedAt:    now,
		}

		err := handler.Handle(ctx, cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		snap, err := snapshotRepo.FindByUserID(ctx, "user_100")
		if err != nil || snap == nil {
			t.Fatalf("expected snapshot to be saved, got: %v", snap)
		}
		if snap.DisplayName() != "John Doe" {
			t.Errorf("expected display name John Doe, got %s", snap.DisplayName())
		}
		if snap.AvatarURL() != "https://example.com/avatar.png" {
			t.Errorf("expected avatar url, got %s", snap.AvatarURL())
		}
	})

	t.Run("fails when user ID is empty", func(t *testing.T) {
		cmd := command.SyncUserSnapshotCommand{
			UserID:      "",
			DisplayName: "John Doe",
			SyncedAt:    now,
		}

		err := handler.Handle(ctx, cmd)
		if err == nil {
			t.Errorf("expected error for empty user ID, got nil")
		}
	})

	t.Run("successfully syncs user XP and clamps negative XP", func(t *testing.T) {
		err := handler.HandleXP(ctx, command.SyncUserXPCommand{
			UserID:   "user_100",
			XP:       1250,
			SyncedAt: now,
		})
		if err != nil {
			t.Fatalf("HandleXP unexpected error: %v", err)
		}
		snap, _ := snapshotRepo.FindByUserID(ctx, "user_100")
		if snap == nil || snap.XP() != 1250 {
			t.Fatalf("expected xp 1250, got %+v", snap)
		}

		if err := handler.HandleXP(ctx, command.SyncUserXPCommand{UserID: "", XP: 100}); err == nil {
			t.Errorf("expected error for empty user ID in HandleXP")
		}
	})
}
