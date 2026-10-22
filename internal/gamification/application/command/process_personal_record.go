package command

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type ProcessPersonalRecordCommand struct {
	UserID       string
	ExerciseID   string
	Weight       float64
	Reps         int32
	ActivityDate string
	AchievedAt   time.Time
}

type ProcessPersonalRecordHandler struct {
	userXpRepo   repository.UserXpRepository
	badgeRepo    repository.BadgeRepository
	txManager    port.TransactionManager
	outboxWriter port.OutboxWriter
}

func NewProcessPersonalRecordHandler(
	userXpRepo repository.UserXpRepository,
	badgeRepo repository.BadgeRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *ProcessPersonalRecordHandler {
	return &ProcessPersonalRecordHandler{
		userXpRepo:   userXpRepo,
		badgeRepo:    badgeRepo,
		txManager:    txManager,
		outboxWriter: outboxWriter,
	}
}

func (h *ProcessPersonalRecordHandler) Handle(ctx context.Context, cmd ProcessPersonalRecordCommand) error {
	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		userXp, err := h.userXpRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("find user xp: %w", err)
		}
		if userXp == nil {
			userXp, err = aggregate.NewUserXp(cmd.UserID)
			if err != nil {
				return fmt.Errorf("create user xp: %w", err)
			}
		}

		xpEvents, err := userXp.EarnXp(25, "PERSONAL_RECORD", cmd.AchievedAt)
		if err != nil {
			return fmt.Errorf("earn pr xp: %w", err)
		}

		if err := h.userXpRepo.Save(txCtx, userXp); err != nil {
			return fmt.Errorf("save user xp: %w", err)
		}

		history := entity.NewXpHistoryEntry(
			uuid.NewString(),
			cmd.UserID,
			25,
			"PERSONAL_RECORD",
			nil,
			cmd.ActivityDate,
			cmd.AchievedAt,
		)
		if err := h.userXpRepo.SaveHistory(txCtx, history); err != nil {
			return fmt.Errorf("save xp history: %w", err)
		}

		outboxEvents := make([]any, len(xpEvents))
		for i, ev := range xpEvents {
			outboxEvents[i] = ev
		}

		if err := h.outboxWriter.WriteEvents(txCtx, "user_xp", cmd.UserID, outboxEvents); err != nil {
			return fmt.Errorf("write outbox events: %w", err)
		}

		has, err := h.badgeRepo.HasBadge(txCtx, cmd.UserID, "PR_BREAKER")
		if err == nil && !has {
			def, _ := vo.GetBadgeDefinition("PR_BREAKER")
			meta := map[string]string{
				"exerciseId": cmd.ExerciseID,
				"weight":     fmt.Sprintf("%.1f", cmd.Weight),
				"reps":       strconv.Itoa(int(cmd.Reps)),
			}
			badge, badgeEv, bErr := aggregate.UnlockBadge(
				uuid.NewString(),
				cmd.UserID,
				"PR_BREAKER",
				def.Name,
				def.Category,
				meta,
				cmd.AchievedAt,
			)
			if bErr == nil {
				if err := h.badgeRepo.Save(txCtx, badge); err == nil {
					_ = h.outboxWriter.WriteEvents(txCtx, "user_badges", cmd.UserID, []any{badgeEv})
				}
			}
		}

		return nil
	})
}
