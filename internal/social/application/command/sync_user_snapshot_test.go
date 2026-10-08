package command

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestSyncUserSnapshotHandler(t *testing.T) {
	repo := &mockUserSnapshotRepo{users: make(map[string]*entity.UserSnapshot)}
	tx := &mockTxManager{}
	handler := NewSyncUserSnapshotHandler(repo, tx)

	err := handler.Handle(context.Background(), SyncUserSnapshotCommand{
		UserID:    "u-123",
		FullName:  "John Doe",
		AvatarURL: "https://avatar.png",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u, _ := repo.GetByID(context.Background(), "u-123")
	if u == nil || u.FullName() != "John Doe" || u.AvatarURL() != "https://avatar.png" {
		t.Errorf("unexpected user snapshot: %+v", u)
	}

	err = handler.Handle(context.Background(), SyncUserSnapshotCommand{
		UserID:    "u-456",
		FullName:  "Jane Doe",
		AvatarURL: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u2, _ := repo.GetByID(context.Background(), "u-456")
	if u2 == nil || u2.AvatarURL() != entity.DefaultAvatarURL {
		t.Errorf("expected default avatar url, got %+v", u2)
	}

	// Validation error: empty user
	err = handler.Handle(context.Background(), SyncUserSnapshotCommand{
		UserID: "",
	})
	if err == nil {
		t.Errorf("expected error on empty user id, got nil")
	}
}
