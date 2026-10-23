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
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/service"
)

type ProcessWorkoutXpCommand struct {
	UserID           string
	SessionID        string
	ActivityDate     string
	TotalSets        int32
	TotalVolumeKg    float64
	VolumeRatio      *float64
	AverageFormScore *float32
	HasNewPR         bool
	CurrentStreak    int32
	CompletedAt      time.Time
}

type ProcessWorkoutXpResult struct {
	DeltaXp  int32
	NewXp    int64
	OldLevel int32
	NewLevel int32
	Events   []event.Event
}

type ProcessWorkoutXpHandler struct {
	userXpRepo   repository.UserXpRepository
	txManager    port.TransactionManager
	outboxWriter port.OutboxWriter
}

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

func (h *ProcessWorkoutXpHandler) HandleEvents(
	ctx context.Context,
	currentStreak int32,
	events []event.Event,
) ([]event.Event, error) {
	if h == nil {
		return nil, nil
	}
	var emitted []event.Event
	for _, ev := range events {
		if e, ok := ev.(*event.WorkoutActivityIngested); ok {
			res, err := h.Handle(ctx, &ProcessWorkoutXpCommand{
				UserID:           e.UserID,
				SessionID:        e.SessionID,
				ActivityDate:     e.ActivityDate,
				TotalSets:        e.TotalSets,
				TotalVolumeKg:    e.TotalVolumeKg,
				VolumeRatio:      e.VolumeRatio,
				AverageFormScore: e.AverageFormScore,
				HasNewPR:         e.HasNewPR,
				CurrentStreak:    currentStreak,
				CompletedAt:      e.CompletedAt,
			})
			if err != nil {
				if errors.Is(err, service.ErrInsufficientWorkout) {
					if h.userXpRepo != nil {
						_, _ = h.userXpRepo.FindByIDForUpdate(ctx, e.UserID)
					}
					continue
				}
				return nil, err
			}
			emitted = append(emitted, res.Events...)
		}
	}
	return emitted, nil
}

func (h *ProcessWorkoutXpHandler) Handle(
	ctx context.Context,
	cmd *ProcessWorkoutXpCommand,
) (*ProcessWorkoutXpResult, error) {
	if cmd == nil {
		return nil, errors.New("process workout xp command is nil")
	}
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
		userXp, findErr := h.userXpRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if findErr != nil {
			return fmt.Errorf("find user_xp for update: %w", findErr)
		}
		if userXp == nil {
			var initErr error
			userXp, initErr = aggregate.NewUserXp(cmd.UserID)
			if initErr != nil {
				return fmt.Errorf("initialize user_xp aggregate: %w", initErr)
			}
		}

		if cmd.SessionID != "" {
			alreadyProcessed, sErr := h.userXpRepo.HasSession(txCtx, cmd.UserID, cmd.SessionID)
			if sErr != nil {
				return fmt.Errorf("check session idempotency: %w", sErr)
			}
			if alreadyProcessed {
				result = ProcessWorkoutXpResult{
					DeltaXp:  0,
					NewXp:    userXp.Xp(),
					OldLevel: userXp.Level(),
					NewLevel: userXp.Level(),
				}
				return nil
			}
		}

		oldLevel := userXp.Level()
		events, earnErr := userXp.EarnXp(deltaXp, "WORKOUT", cmd.CompletedAt)
		if earnErr != nil {
			return fmt.Errorf("earn workout xp: %w", earnErr)
		}

		if saveErr := h.userXpRepo.Save(txCtx, userXp); saveErr != nil {
			return fmt.Errorf("save user_xp: %w", saveErr)
		}

		var sessionPtr *string
		if cmd.SessionID != "" {
			s := cmd.SessionID
			sessionPtr = &s
		}

		history := entity.NewXpHistoryEntry(
			uuid.NewString(),
			cmd.UserID,
			deltaXp,
			"WORKOUT",
			sessionPtr,
			cmd.ActivityDate,
			cmd.CompletedAt,
		)
		if histErr := h.userXpRepo.SaveHistory(txCtx, history); histErr != nil {
			return fmt.Errorf("save xp_history: %w", histErr)
		}

		evAnys := make([]any, len(events))
		for i, ev := range events {
			evAnys[i] = ev
		}

		if writeErr := h.outboxWriter.WriteEvents(txCtx, "user_xp", cmd.UserID, evAnys); writeErr != nil {
			return fmt.Errorf("write outbox events: %w", writeErr)
		}

		result = ProcessWorkoutXpResult{
			DeltaXp:  deltaXp,
			NewXp:    userXp.Xp(),
			OldLevel: oldLevel,
			NewLevel: userXp.Level(),
			Events:   events,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}
