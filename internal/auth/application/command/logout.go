package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// LogoutCommand represents the command to log out a session.
type LogoutCommand struct {
	RefreshToken string
	UserID       string
}

// LogoutHandler processes token revocations and invalidates user sessions.
type LogoutHandler struct {
	sessRepo port.SessionRepository
}

// NewLogoutHandler constructs a LogoutHandler instance.
func NewLogoutHandler(sessRepo port.SessionRepository) *LogoutHandler {
	return &LogoutHandler{sessRepo: sessRepo}
}

// Handle invalidates the refresh token session in the database.
func (h *LogoutHandler) Handle(ctx context.Context, cmd LogoutCommand) error {
	sess, err := h.sessRepo.FindByToken(ctx, cmd.RefreshToken)
	if err != nil {
		return fmt.Errorf("session lookup: %w", err)
	}

	if sess.UserID != cmd.UserID {
		return apperror.ErrUnauthorized
	}

	if err := h.sessRepo.Delete(ctx, cmd.RefreshToken); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}
