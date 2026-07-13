package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// RefreshTokenCommand represents the command to generate a new access token using a refresh token.
type RefreshTokenCommand struct {
	RefreshToken string
}

// RefreshTokenResult holds the generated token response details.
type RefreshTokenResult struct {
	AccessToken  string
	RefreshToken string
}

// RefreshTokenHandler processes refresh token requests in the application layer.
type RefreshTokenHandler struct {
	userRepo  repository.UserRepository
	keyRepo   port.KeyRepository
	sessRepo  port.SessionRepository
	tokenServ port.TokenService
}

// NewRefreshTokenHandler constructs a new RefreshTokenHandler.
func NewRefreshTokenHandler(
	userRepo repository.UserRepository,
	keyRepo port.KeyRepository,
	sessRepo port.SessionRepository,
	tokenServ port.TokenService,
) *RefreshTokenHandler {
	return &RefreshTokenHandler{
		userRepo:  userRepo,
		keyRepo:   keyRepo,
		sessRepo:  sessRepo,
		tokenServ: tokenServ,
	}
}

// Handle validates the refresh token, cleans up expired session if needed, and issues a new access token.
func (h *RefreshTokenHandler) Handle(ctx context.Context, cmd RefreshTokenCommand) (*RefreshTokenResult, error) {
	// 1. Find session
	sess, err := h.sessRepo.FindByToken(ctx, cmd.RefreshToken)
	if err != nil {
		return nil, apperror.ErrUnauthorized
	}

	if sess.IsExpired() {
		// Delete expired session to clean up
		_ = h.sessRepo.Delete(ctx, cmd.RefreshToken)
		return nil, apperror.ErrUnauthorized
	}

	// 2. Find user
	user, err := h.userRepo.FindByID(ctx, sess.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	// 3. Find active key
	activeKey, err := h.keyRepo.GetActiveKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("active signing key not found: %w", err)
	}

	// 4. Generate new access token
	newAccessToken, _, err := h.tokenServ.GenerateAccessToken(ctx, user, activeKey.ID)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	return &RefreshTokenResult{
		AccessToken:  newAccessToken,
		RefreshToken: cmd.RefreshToken,
	}, nil
}
