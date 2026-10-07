package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
)

func TestUpdateUserRoleHandler(t *testing.T) {
	t.Run("success update user role", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{users: make(map[string]*entity.UserSnapshot)}
		repo.users["u-1"] = entity.NewUserSnapshot("u-1", "Test User", "https://avatar.png", "user", time.Now())
		tx := &mockTxManager{}
		handler := NewUpdateUserRoleHandler(repo, tx)

		err := handler.Handle(context.Background(), UpdateUserRoleCommand{
			UserID: "u-1",
			Role:   "brand",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.users["u-1"].Role() != "brand" {
			t.Errorf("expected role 'brand', got %s", repo.users["u-1"].Role())
		}
	})

	t.Run("validation error empty user", func(t *testing.T) {
		handler := NewUpdateUserRoleHandler(nil, nil)
		err := handler.Handle(context.Background(), UpdateUserRoleCommand{
			UserID: "",
			Role:   "brand",
		})
		if !errors.Is(err, derror.ErrUnauthorized) {
			t.Errorf("got %v, want ErrUnauthorized", err)
		}
	})

	t.Run("validation error empty role", func(t *testing.T) {
		handler := NewUpdateUserRoleHandler(nil, nil)
		err := handler.Handle(context.Background(), UpdateUserRoleCommand{
			UserID: "u-1",
			Role:   "",
		})
		if err == nil {
			t.Errorf("expected error on empty role, got nil")
		}
	})
}
