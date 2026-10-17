package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/service"
)

// ProcessNutritionXpCommand carries nutrition adherence details.
type ProcessNutritionXpCommand struct {
	UserID       string
	ActivityDate string // YYYY-MM-DD
	AchievedAt   time.Time
}

// ProcessNutritionXpResult contains the outcome of awarding nutrition XP.
type ProcessNutritionXpResult struct {
	Claimed  bool
	DeltaXp  int32
	NewXp    int64
	NewLevel int32
}

// ProcessNutritionXpHandler awards fixed +30 XP for meeting daily nutrition targets.
type ProcessNutritionXpHandler struct {
	userXpRepo   repository.UserXpRepository
	txManager    port.TransactionManager
	outboxWriter port.OutboxWriter
}

// NewProcessNutritionXpHandler constructs ProcessNutritionXpHandler.
func NewProcessNutritionXpHandler(
	userXpRepo repository.UserXpRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *ProcessNutritionXpHandler {
	return &ProcessNutritionXpHandler{
		userXpRepo:   userXpRepo,
		txManager:    txManager,
		outboxWriter: outboxWriter,
	}
}

// Handle awards the daily nutrition bonus if not already claimed for the local date.
func (h *ProcessNutritionXpHandler) Handle(
	ctx context.Context,
	cmd ProcessNutritionXpCommand,
) (*ProcessNutritionXpResult, error) {
	var result ProcessNutritionXpResult

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		userXp, err := h.userXpRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("find user_xp for update: %w", err)
		}
		if userXp == nil {
			userXp, err = aggregate.NewUserXp(cmd.UserID)
			if err != nil {
				return fmt.Errorf("initialize user_xp aggregate: %w", err)
			}
		}

		events, err := userXp.EarnNutritionXp(cmd.ActivityDate, cmd.AchievedAt)
		if err != nil {
			if errors.Is(err, aggregate.ErrNutritionRewardAlreadyClaimedToday) {
				// Idempotent no-op: already rewarded today
				result = ProcessNutritionXpResult{
					Claimed:  false,
					DeltaXp:  0,
					NewXp:    userXp.Xp(),
					NewLevel: userXp.Level(),
				}
				return nil
			}
			return fmt.Errorf("earn nutrition xp: %w", err)
		}

		if err := h.userXpRepo.Save(txCtx, userXp); err != nil {
			return fmt.Errorf("save user_xp: %w", err)
		}

		history := entity.NewXpHistoryEntry(
			uuid.NewString(),
			cmd.UserID,
			service.NutritionDailyXpReward,
			"NUTRITION",
			nil,
			cmd.ActivityDate,
			cmd.AchievedAt,
		)
		if err := h.userXpRepo.SaveHistory(txCtx, history); err != nil {
			return fmt.Errorf("save nutrition xp_history: %w", err)
		}

		evAnys := make([]any, len(events))
		for i, ev := range events {
			evAnys[i] = ev
		}

		if err := h.outboxWriter.WriteEvents(txCtx, "user_xp", cmd.UserID, evAnys); err != nil {
			return fmt.Errorf("write outbox events: %w", err)
		}

		result = ProcessNutritionXpResult{
			Claimed:  true,
			DeltaXp:  service.NutritionDailyXpReward,
			NewXp:    userXp.Xp(),
			NewLevel: userXp.Level(),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &result, nil
}
