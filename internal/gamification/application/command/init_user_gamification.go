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
		userStreakRepo:   userStreakRepo,
		leaderboardRepo:  leaderboardRepo,
		txManager:       txManager,
	}
}

// Handle sets up initial UserXp, UserStreak, and UserSnapshot records.
func (h *InitUserGamificationHandler) Handle(ctx context.Context, cmd InitUserGamificationCommand) error {
	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		existingXp, err := h.userXpRepo.FindByID(txCtx, cmd.UserID)
		if err != nil {
			return err
		}
		if existingXp == nil {
			newXp, err := aggregate.NewUserXp(cmd.UserID)
			if err != nil {
				return err
			}
			if err := h.userXpRepo.Save(txCtx, newXp); err != nil {
				return err
			}
		}

		existingStreak, err := h.userStreakRepo.FindByID(txCtx, cmd.UserID)
		if err != nil {
			return err
		}
		if existingStreak == nil {
			newStreak, err := aggregate.NewUserStreak(cmd.UserID)
			if err != nil {
				return err
			}
			if err := h.userStreakRepo.Save(txCtx, newStreak); err != nil {
				return err
			}
		}

		displayName := cmd.DisplayName
		if displayName == "" {
			displayName = "Gymer"
		}
		snapshot := entity.NewUserSnapshot(cmd.UserID, displayName, cmd.AvatarURL, time.Now().UTC())
		if err := h.leaderboardRepo.SaveSnapshot(txCtx, snapshot); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("init user gamification: %w", err)
	}

	return nil
}
