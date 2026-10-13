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
	"github.com/viethung213/gym-companion/internal/gamification/domain/service"
)

// ProcessWorkoutXpCommand carries workout details to reward XP.
type ProcessWorkoutXpCommand struct {
	UserID           string
	SessionID        string
	ActivityDate     string // YYYY-MM-DD
	TotalSets        int32
	TotalVolumeKg    float64
	VolumeRatio      *float64
	AverageFormScore *float32
	HasNewPR         bool
	CurrentStreak    int32
	CompletedAt      time.Time
}

// ProcessWorkoutXpResult contains the outcome of processing workout XP.
type ProcessWorkoutXpResult struct {
	DeltaXp  int32
	NewXp    int64
	OldLevel int32
	NewLevel int32
}

// ProcessWorkoutXpHandler coordinates awarding XP for a completed workout in a short local transaction.
type ProcessWorkoutXpHandler struct {
	userXpRepo   repository.UserXpRepository
	txManager    port.TransactionManager
	outboxWriter port.OutboxWriter
}

// NewProcessWorkoutXpHandler constructs ProcessWorkoutXpHandler.
func NewProcessWorkoutXpHandler(
	userXpRepo repository.UserXpRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *ProcessWorkoutXpHandler {
	return &ProcessWorkoutXpHandler{
		userXpRepo:   userXpRepo,
		txManager:    txManager,
		outboxWriter: outboxWriter,
	}
}

// Handle executes the command inside a pessimistic locking transaction.
func (h *ProcessWorkoutXpHandler) Handle(
	ctx context.Context,
	cmd ProcessWorkoutXpCommand,
) (*ProcessWorkoutXpResult, error) {
	deltaXp, err := service.CalculateWorkoutXp(service.WorkoutXpInput{
		TotalSets:        cmd.TotalSets,
		TotalVolumeKg:    cmd.TotalVolumeKg,
		VolumeRatio:      cmd.VolumeRatio,
		AverageFormScore: cmd.AverageFormScore,
		HasNewPR:         cmd.HasNewPR,
		CurrentStreak:    cmd.CurrentStreak,
	})
	if err != nil {
		return nil, fmt.Errorf("calculate workout xp: %w", err)
	}

	var result ProcessWorkoutXpResult

	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if cmd.SessionID != "" {
			alreadyProcessed, sErr := h.userXpRepo.HasSession(txCtx, cmd.UserID, cmd.SessionID)
			if sErr != nil {
				return fmt.Errorf("check session idempotency: %w", sErr)
			}
			if alreadyProcessed {
				userXp, fErr := h.userXpRepo.FindByID(txCtx, cmd.UserID)
				if fErr != nil {
					return fmt.Errorf("find user_xp for idempotent return: %w", fErr)
				}
				if userXp != nil {
					result = ProcessWorkoutXpResult{
						DeltaXp:  0,
						NewXp:    userXp.Xp(),
						NewLevel: userXp.Level(),
					}
					return nil
				}
			}
		}

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

		oldLevel := userXp.Level()
		events, err := userXp.EarnXp(deltaXp, "WORKOUT", cmd.CompletedAt)
		if err != nil {
			return fmt.Errorf("earn xp: %w", err)
		}

		if err := h.userXpRepo.Save(txCtx, userXp); err != nil {
			return fmt.Errorf("save user_xp: %w", err)
		}

		sessionID := cmd.SessionID
		history := entity.NewXpHistoryEntry(
			uuid.NewString(),
			cmd.UserID,
			deltaXp,
			"WORKOUT",
			&sessionID,
			cmd.ActivityDate,
			cmd.CompletedAt,
		)
		if err := h.userXpRepo.SaveHistory(txCtx, history); err != nil {
			return fmt.Errorf("save xp_history: %w", err)
		}

		evAnys := make([]any, len(events))
		for i, ev := range events {
			evAnys[i] = ev
		}

		if err := h.outboxWriter.WriteEvents(txCtx, "user_xp", cmd.UserID, evAnys); err != nil {
			return fmt.Errorf("write outbox events: %w", err)
		}

		result = ProcessWorkoutXpResult{
			DeltaXp:  deltaXp,
			NewXp:    userXp.Xp(),
			OldLevel: oldLevel,
			NewLevel: userXp.Level(),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &result, nil
}
