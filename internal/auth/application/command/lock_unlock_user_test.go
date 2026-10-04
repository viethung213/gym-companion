//go:build unit

package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

func TestLockUserHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully lock active user and purge sessions", func(t *testing.T) {
		userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
		sessionRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}

		ident := aggregate.NewIdentity("id-1", aggregate.IdentityTypeEmail, "user@fitai.com", "", nil, time.Now(), time.Now())
		user := aggregate.RegisterUser("user-1", "Regular User", ident, "")
		_ = userRepo.Create(ctx, user)

		// Create active session
		_ = sessionRepo.Save(ctx, "session-token-1", "user-1", time.Now().Add(24*time.Hour))

		handler := NewLockUserHandler(userRepo, sessionRepo)
		err := handler.Handle(ctx, LockUserCommand{
			AdminID: "admin-1",
			UserID:  "user-1",
			Reason:  "Violation of terms",
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		updatedUser, _ := userRepo.FindByID(ctx, "user-1")
		if got, want := updatedUser.Status(), aggregate.UserStatusLocked; got != want {
			t.Errorf("got status %s, want %s", got, want)
		}

		if len(sessionRepo.sessions) != 0 {
			t.Errorf("expected 0 sessions after lock, got %d", len(sessionRepo.sessions))
		}
	})

	t.Run("cannot lock an admin user", func(t *testing.T) {
		userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
		sessionRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}

		adminRole, _ := vo.NewRole("admin")
		ident := aggregate.NewIdentity("id-admin", aggregate.IdentityTypeEmail, "admin@fitai.com", "", nil, time.Now(), time.Now())
		adminUser := aggregate.NewUser("admin-2", "Admin User", adminRole, aggregate.UserStatusActive, ident, time.Now(), time.Now())
		_ = userRepo.Create(ctx, adminUser)

		handler := NewLockUserHandler(userRepo, sessionRepo)
		err := handler.Handle(ctx, LockUserCommand{
			AdminID: "admin-1",
			UserID:  "admin-2",
		})
		if err != derror.ErrCannotLockAdmin {
			t.Errorf("expected ErrCannotLockAdmin, got %v", err)
		}
	})

	t.Run("cannot lock an already locked user", func(t *testing.T) {
		userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
		sessionRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}

		ident := aggregate.NewIdentity("id-1", aggregate.IdentityTypeEmail, "user@fitai.com", "", nil, time.Now(), time.Now())
		user := aggregate.RegisterUser("user-1", "Regular User", ident, "")
		_ = user.Lock()
		_ = userRepo.Create(ctx, user)

		handler := NewLockUserHandler(userRepo, sessionRepo)
		err := handler.Handle(ctx, LockUserCommand{
			AdminID: "admin-1",
			UserID:  "user-1",
		})
		if err != derror.ErrUserAlreadyLocked {
			t.Errorf("expected ErrUserAlreadyLocked, got %v", err)
		}
	})

	t.Run("fails when user does not exist", func(t *testing.T) {
		userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
		sessionRepo := &mockSessionRepo{sessions: make(map[string]*port.SessionRecord)}

		handler := NewLockUserHandler(userRepo, sessionRepo)
		err := handler.Handle(ctx, LockUserCommand{
			AdminID: "admin-1",
			UserID:  "non-existent",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestUnlockUserHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully unlock a locked user", func(t *testing.T) {
		userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}

		ident := aggregate.NewIdentity("id-1", aggregate.IdentityTypeEmail, "user@fitai.com", "", nil, time.Now(), time.Now())
		user := aggregate.RegisterUser("user-1", "Regular User", ident, "")
		_ = user.Lock()
		_ = userRepo.Create(ctx, user)

		handler := NewUnlockUserHandler(userRepo)
		err := handler.Handle(ctx, UnlockUserCommand{
			AdminID: "admin-1",
			UserID:  "user-1",
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		updatedUser, _ := userRepo.FindByID(ctx, "user-1")
		if got, want := updatedUser.Status(), aggregate.UserStatusActive; got != want {
			t.Errorf("got status %s, want %s", got, want)
		}
	})

	t.Run("cannot unlock an already active user", func(t *testing.T) {
		userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}

		ident := aggregate.NewIdentity("id-1", aggregate.IdentityTypeEmail, "user@fitai.com", "", nil, time.Now(), time.Now())
		user := aggregate.RegisterUser("user-1", "Regular User", ident, "")
		_ = userRepo.Create(ctx, user)

		handler := NewUnlockUserHandler(userRepo)
		err := handler.Handle(ctx, UnlockUserCommand{
			AdminID: "admin-1",
			UserID:  "user-1",
		})
		if err != derror.ErrUserAlreadyActive {
			t.Errorf("expected ErrUserAlreadyActive, got %v", err)
		}
	})

	t.Run("fails when user does not exist", func(t *testing.T) {
		userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}

		handler := NewUnlockUserHandler(userRepo)
		err := handler.Handle(ctx, UnlockUserCommand{
			AdminID: "admin-1",
			UserID:  "non-existent",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
