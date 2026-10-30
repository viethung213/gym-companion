package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

type RecordStreakActivityCommand struct {
	UserID       string
	ActivityDate string
	ActivityType string
	RecordedAt   time.Time
}

type RecordStreakActivityResult struct {
	CurrentStreak int32
	LongestStreak int32
	Advanced      bool
	Events        []event.Event
}

type RecordStreakActivityHandler struct {
	userStreakRepo repository.UserStreakRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
}

func NewRecordStreakActivityHandler(
	userStreakRepo repository.UserStreakRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *RecordStreakActivityHandler {
	return &RecordStreakActivityHandler{
		userStreakRepo: userStreakRepo,
		txManager:      txManager,
		outboxWriter:   outboxWriter,
	}
}

func (h *RecordStreakActivityHandler) HandleEvents(
	ctx context.Context,
	events []event.Event,
) ([]event.Event, error) {
	if h == nil {
		return nil, nil
	}
	var emitted []event.Event
	for _, ev := range events {
		switch e := ev.(type) {
		case *event.WorkoutActivityIngested:
			res, err := h.Handle(ctx, RecordStreakActivityCommand{
				UserID:       e.UserID,
				ActivityDate: e.ActivityDate,
				ActivityType: "WORKOUT",
				RecordedAt:   e.CompletedAt,
			})
			if err != nil {
				return nil, err
			}
			emitted = append(emitted, res.Events...)
		case *event.NutritionActivityIngested:
			res, err := h.Handle(ctx, RecordStreakActivityCommand{
				UserID:       e.UserID,
				ActivityDate: e.ActivityDate,
				ActivityType: "NUTRITION",
				RecordedAt:   e.RecordedAt,
			})
			if err != nil {
				return nil, err
			}
			emitted = append(emitted, res.Events...)
		}
	}
	return emitted, nil
}

func (h *RecordStreakActivityHandler) Handle(
	ctx context.Context,
	cmd RecordStreakActivityCommand,
) (*RecordStreakActivityResult, error) {
	var result RecordStreakActivityResult

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		userStreak, err := h.userStreakRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("find user_streak for update: %w", err)
		}
		if userStreak == nil {
			userStreak, err = aggregate.NewUserStreak(cmd.UserID)
			if err != nil {
				return fmt.Errorf("initialize user_streak aggregate: %w", err)
			}
		}

		events, err := userStreak.RecordActivity(cmd.ActivityDate, cmd.ActivityType, cmd.RecordedAt)
		if err != nil {
			return fmt.Errorf("record streak activity: %w", err)
		}

		if len(events) == 0 {
			result = RecordStreakActivityResult{
				CurrentStreak: userStreak.CurrentStreak(),
				LongestStreak: userStreak.LongestStreak(),
				Advanced:      false,
				Events:        nil,
			}
			return nil
		}

		if err := h.userStreakRepo.Save(txCtx, userStreak); err != nil {
			return fmt.Errorf("save user_streak: %w", err)
		}

		history := entity.NewStreakHistoryEntry(
			uuid.NewString(),
			cmd.UserID,
			cmd.ActivityDate,
			cmd.ActivityType,
			cmd.RecordedAt,
		)
		if err := h.userStreakRepo.SaveHistory(txCtx, history); err != nil {
			return fmt.Errorf("save streak_history: %w", err)
		}

		evAnys := make([]any, len(events))
		for i, ev := range events {
			evAnys[i] = ev
		}

		if err := h.outboxWriter.WriteEvents(txCtx, "user_streak", cmd.UserID, evAnys); err != nil {
			return fmt.Errorf("write outbox events: %w", err)
		}

		result = RecordStreakActivityResult{
			CurrentStreak: userStreak.CurrentStreak(),
			LongestStreak: userStreak.LongestStreak(),
			Advanced:      true,
			Events:        events,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}
