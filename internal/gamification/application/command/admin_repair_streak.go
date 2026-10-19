package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// AdminRepairStreakCommand contains parameters to directly set a user's streak.
type AdminRepairStreakCommand struct {
	UserID        string
	AdminID       string
	TargetStreak  int32
	Reason        string
	EffectiveDate string
}

// AdminRepairStreakResult returns the updated streak state.
type AdminRepairStreakResult struct {
	UserID        string
	CurrentStreak int32
	LongestStreak int32
}

// AdminRepairStreakHandler handles administrative streak repair.
type AdminRepairStreakHandler struct {
	userStreakRepo repository.UserStreakRepository
	txManager      port.TransactionManager
}

// NewAdminRepairStreakHandler constructs AdminRepairStreakHandler.
func NewAdminRepairStreakHandler(
	userStreakRepo repository.UserStreakRepository,
	txManager port.TransactionManager,
) *AdminRepairStreakHandler {
	return &AdminRepairStreakHandler{
		userStreakRepo: userStreakRepo,
		txManager:      txManager,
	}
}

// Handle executes the streak repair atomically.
func (h *AdminRepairStreakHandler) Handle(ctx context.Context, cmd AdminRepairStreakCommand) (*AdminRepairStreakResult, error) {
	if cmd.UserID == "" {
		return nil, aggregate.ErrEmptyUserID
	}
	if _, err := uuid.Parse(cmd.UserID); err != nil {
		return nil, aggregate.ErrInvalidUserID
	}
	if cmd.TargetStreak < 0 {
		return nil, aggregate.ErrNegativeStreak
	}

	var result *AdminRepairStreakResult
	now := time.Now().UTC()

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		streak, err := h.userStreakRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("lock user_streak: %w", err)
		}
		if streak == nil {
			streak, err = aggregate.NewUserStreak(cmd.UserID)
			if err != nil {
				return err
			}
		}

		if err := streak.RepairStreakByAdmin(cmd.TargetStreak, cmd.EffectiveDate, now); err != nil {
			return err
		}

		if err := h.userStreakRepo.Save(txCtx, streak); err != nil {
			return fmt.Errorf("save user_streak: %w", err)
		}

		if streak.LastActivityDate() != "" {
			history := entity.NewStreakHistoryEntry(
				uuid.NewString(),
				streak.UserID(),
				streak.LastActivityDate(),
				"ADMIN_REPAIR",
				now,
			)
			if err := h.userStreakRepo.SaveHistory(txCtx, history); err != nil {
				return fmt.Errorf("save streak_history: %w", err)
			}
		}

		result = &AdminRepairStreakResult{
			UserID:        streak.UserID(),
			CurrentStreak: streak.CurrentStreak(),
			LongestStreak: streak.LongestStreak(),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
