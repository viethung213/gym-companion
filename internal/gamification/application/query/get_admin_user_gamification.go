package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// GetAdminUserGamificationQuery carries the user identifier to inspect.
type GetAdminUserGamificationQuery struct {
	UserID string
}

// AdminUserGamificationDTO represents the unified gamification profile for admin inspection.
type AdminUserGamificationDTO struct {
	UserID         string
	Xp             int64
	Level          int32
	CurrentStreak  int32
	LongestStreak  int32
	CoinBalance    int64
	LastActiveDate string
	UpdatedAt      time.Time
}

// GetAdminUserGamificationHandler aggregates user gamification metrics across sub-domains.
type GetAdminUserGamificationHandler struct {
	userXpRepo     repository.UserXpRepository
	userStreakRepo repository.UserStreakRepository
	userWalletRepo repository.UserWalletRepository
}

// NewGetAdminUserGamificationHandler constructs GetAdminUserGamificationHandler.
func NewGetAdminUserGamificationHandler(
	userXpRepo repository.UserXpRepository,
	userStreakRepo repository.UserStreakRepository,
	userWalletRepo repository.UserWalletRepository,
) *GetAdminUserGamificationHandler {
	return &GetAdminUserGamificationHandler{
		userXpRepo:     userXpRepo,
		userStreakRepo: userStreakRepo,
		userWalletRepo: userWalletRepo,
	}
}

// Handle fetches and merges XP, Streak, and Wallet data.
func (h *GetAdminUserGamificationHandler) Handle(ctx context.Context, q GetAdminUserGamificationQuery) (*AdminUserGamificationDTO, error) {
	if strings.TrimSpace(q.UserID) == "" {
		return nil, aggregate.ErrEmptyUserID
	}
	if _, err := uuid.Parse(strings.TrimSpace(q.UserID)); err != nil {
		return nil, aggregate.ErrInvalidUserID
	}

	dto := &AdminUserGamificationDTO{
		UserID:    q.UserID,
		Level:     1,
		UpdatedAt: time.Now().UTC(),
	}

	xp, err := h.userXpRepo.FindByID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user xp: %w", err)
	}
	if xp != nil {
		dto.Xp = xp.Xp()
		dto.Level = xp.Level()
		dto.UpdatedAt = xp.UpdatedAt()
	}

	streak, err := h.userStreakRepo.FindByID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user streak: %w", err)
	}
	if streak != nil {
		dto.CurrentStreak = streak.CurrentStreak()
		dto.LongestStreak = streak.LongestStreak()
		dto.LastActiveDate = streak.LastActivityDate()
		if streak.UpdatedAt().After(dto.UpdatedAt) {
			dto.UpdatedAt = streak.UpdatedAt()
		}
	}

	wallet, err := h.userWalletRepo.FindByID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user wallet: %w", err)
	}
	if wallet != nil {
		dto.CoinBalance = wallet.Balance()
		if wallet.UpdatedAt().After(dto.UpdatedAt) {
			dto.UpdatedAt = wallet.UpdatedAt()
		}
	}

	return dto, nil
}
