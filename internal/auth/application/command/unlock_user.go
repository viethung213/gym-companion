package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// UnlockUserCommand specifies the data needed by an admin to unlock a user account.
type UnlockUserCommand struct {
	AdminID string
	UserID  string
}

// UnlockUserHandler handles the unlocking of a user account by an administrator.
type UnlockUserHandler struct {
	userRepo repository.UserRepository
}

// NewUnlockUserHandler creates a new instance of UnlockUserHandler.
func NewUnlockUserHandler(userRepo repository.UserRepository) *UnlockUserHandler {
	return &UnlockUserHandler{
		userRepo: userRepo,
	}
}

// Handle unlocks the user account and restores it to active status.
func (h *UnlockUserHandler) Handle(ctx context.Context, cmd UnlockUserCommand) error {
	if cmd.UserID == "" {
		return derror.ErrUserNotFound
	}

	user, err := h.userRepo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("find user for unlock: %w", err)
	}
	if user == nil {
		return derror.ErrUserNotFound
	}

	if err := user.Activate(); err != nil {
		return err
	}

	if err := h.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("update user status to active: %w", err)
	}

	return nil
}
