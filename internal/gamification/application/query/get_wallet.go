package query

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// GetWalletQuery defines query parameters for fetching a user's FitCoins wallet summary.
type GetWalletQuery struct {
	UserID string
}

// GetWalletResult contains the user's FitCoins wallet balance and last modification timestamp.
type GetWalletResult struct {
	UserID    string
	Balance   int64
	UpdatedAt time.Time
}

// GetWalletHandler handles GetWalletQuery.
type GetWalletHandler struct {
	userWalletRepo repository.UserWalletRepository
}

// NewGetWalletHandler constructs GetWalletHandler.
func NewGetWalletHandler(userWalletRepo repository.UserWalletRepository) *GetWalletHandler {
	return &GetWalletHandler{userWalletRepo: userWalletRepo}
}

// Handle fetches the user's FitCoins wallet or returns a baseline 0-balance result if not yet persisted.
func (h *GetWalletHandler) Handle(ctx context.Context, q GetWalletQuery) (*GetWalletResult, error) {
	wallet, err := h.userWalletRepo.FindByID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user_wallet: %w", err)
	}

	if wallet == nil {
		return &GetWalletResult{
			UserID:    q.UserID,
			Balance:   0,
			UpdatedAt: time.Now().UTC(),
		}, nil
	}

	return &GetWalletResult{
		UserID:    wallet.UserID(),
		Balance:   wallet.Balance(),
		UpdatedAt: wallet.UpdatedAt(),
	}, nil
}
