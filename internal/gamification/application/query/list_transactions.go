package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// ListTransactionsQuery defines cursor-paginated parameters for querying a user's FitCoins ledger.
type ListTransactionsQuery struct {
	UserID string
	Limit  int
	Cursor string
}

// ListTransactionsResult contains the page of ledger entries and the next cursor token.
type ListTransactionsResult struct {
	Entries    []*entity.CoinLedgerEntry
	NextCursor string
}

// ListTransactionsHandler handles ListTransactionsQuery.
type ListTransactionsHandler struct {
	userWalletRepo repository.UserWalletRepository
}

// NewListTransactionsHandler constructs ListTransactionsHandler.
func NewListTransactionsHandler(userWalletRepo repository.UserWalletRepository) *ListTransactionsHandler {
	return &ListTransactionsHandler{userWalletRepo: userWalletRepo}
}

// Handle returns cursor-paginated FitCoins ledger entries for the user.
func (h *ListTransactionsHandler) Handle(
	ctx context.Context,
	q ListTransactionsQuery,
) (*ListTransactionsResult, error) {
	entries, nextCursor, err := h.userWalletRepo.ListLedgerEntries(ctx, q.UserID, q.Limit, q.Cursor)
	if err != nil {
		return nil, fmt.Errorf("list coin_ledger entries: %w", err)
	}

	return &ListTransactionsResult{
		Entries:    entries,
		NextCursor: nextCursor,
	}, nil
}
