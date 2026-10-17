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

// Handle sets up initial UserXp, UserStreak, and UserSnapshot records.
func (h *InitUserGamificationHandler) Handle(ctx context.Context, cmd InitUserGamificationCommand) error {
	txErr := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
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
