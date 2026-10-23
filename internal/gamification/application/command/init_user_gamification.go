package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// InitUserGamificationCommand initializes a fresh profile upon user registration.
type InitUserGamificationCommand struct {
	UserID      string
	DisplayName string
	AvatarURL   string
}

// InitUserGamificationHandler creates baseline records for a new user.
type InitUserGamificationHandler struct {
	userXpRepo      repository.UserXpRepository
	userStreakRepo  repository.UserStreakRepository
	leaderboardRepo repository.LeaderboardRepository
	userWalletRepo  repository.UserWalletRepository
	txManager       port.TransactionManager
}

// NewInitUserGamificationHandler constructs InitUserGamificationHandler.
func NewInitUserGamificationHandler(
	userXpRepo repository.UserXpRepository,
	userStreakRepo repository.UserStreakRepository,
	leaderboardRepo repository.LeaderboardRepository,
	txManager port.TransactionManager,
) *InitUserGamificationHandler {
	return &InitUserGamificationHandler{
		userXpRepo:      userXpRepo,
		userStreakRepo:  userStreakRepo,
		leaderboardRepo: leaderboardRepo,
		txManager:       txManager,
	}
}

// WithWalletRepository attaches an optional UserWalletRepository for eager wallet initialization.
func (h *InitUserGamificationHandler) WithWalletRepository(
	userWalletRepo repository.UserWalletRepository,
) *InitUserGamificationHandler {
	h.userWalletRepo = userWalletRepo
	return h
}

// Handle sets up initial UserStreak, UserXp, UserWallet, and UserSnapshot records.
// Lock acquisition order strictly adheres to user_streak -> user_xp -> user_wallet to eliminate
// deadlock cycles with ActivityIngestionCoordinator.
func (h *InitUserGamificationHandler) Handle(ctx context.Context, cmd InitUserGamificationCommand) error {
	txErr := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		existingStreak, findStreakErr := h.userStreakRepo.FindByID(txCtx, cmd.UserID)
		if findStreakErr != nil {
			return findStreakErr
		}
		if existingStreak == nil {
			newStreak, createStreakErr := aggregate.NewUserStreak(cmd.UserID)
			if createStreakErr != nil {
				return createStreakErr
			}
			if saveStreakErr := h.userStreakRepo.Save(txCtx, newStreak); saveStreakErr != nil {
				return saveStreakErr
			}
		}

		existingXp, findXpErr := h.userXpRepo.FindByID(txCtx, cmd.UserID)
		if findXpErr != nil {
			return findXpErr
		}
		if existingXp == nil {
			newXp, createXpErr := aggregate.NewUserXp(cmd.UserID)
			if createXpErr != nil {
				return createXpErr
			}
			if saveXpErr := h.userXpRepo.Save(txCtx, newXp); saveXpErr != nil {
				return saveXpErr
			}
		}

		if h.userWalletRepo != nil {
			existingWallet, findWalletErr := h.userWalletRepo.FindByID(txCtx, cmd.UserID)
			if findWalletErr != nil {
				return findWalletErr
			}
			if existingWallet == nil {
				newWallet, createWalletErr := aggregate.NewUserWallet(cmd.UserID)
				if createWalletErr != nil {
					return createWalletErr
				}
				if saveWalletErr := h.userWalletRepo.Save(txCtx, newWallet); saveWalletErr != nil {
					return saveWalletErr
				}
			}
		}

		displayName := cmd.DisplayName
		if displayName == "" {
			displayName = "Gymer"
		}
		snapshot := entity.NewUserSnapshot(cmd.UserID, displayName, cmd.AvatarURL, time.Now().UTC())
		return h.leaderboardRepo.SaveSnapshot(txCtx, snapshot)
	})

	if txErr != nil {
		return fmt.Errorf("init user gamification: %w", txErr)
	}

	return nil
}
