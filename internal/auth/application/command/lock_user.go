package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// LockUserCommand specifies the data needed by an admin to lock a user account.
type LockUserCommand struct {
	AdminID string
	UserID  string
	Reason  string
}

// LockUserHandler handles the locking of a user account by an administrator.
type LockUserHandler struct {
	userRepo    repository.UserRepository
	sessionRepo port.SessionRepository
}

// NewLockUserHandler creates a new instance of LockUserHandler.
func NewLockUserHandler(
	userRepo repository.UserRepository,
	sessionRepo port.SessionRepository,
) *LockUserHandler {
	return &LockUserHandler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

// Handle locks the user account, updates the database, and invalidates all active sessions.
func (h *LockUserHandler) Handle(ctx context.Context, cmd LockUserCommand) error {
	if cmd.UserID == "" {
		return derror.ErrUserNotFound
	}

	user, err := h.userRepo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("find user for lock: %w", err)
	}
	if user == nil {
		return derror.ErrUserNotFound
	}

	if err := user.Lock(); err != nil {
		return err
	}

	if err := h.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("update user status to locked: %w", err)
	}

	// Revoke active sessions for security
	if h.sessionRepo != nil {
		_ = h.sessionRepo.DeleteAllByUserID(ctx, user.ID())
	}

	return nil
}
