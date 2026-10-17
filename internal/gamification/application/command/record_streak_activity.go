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

// RecordStreakActivityCommand carries parameters to record a daily active streak entry.
type RecordStreakActivityCommand struct {
	UserID       string
	ActivityDate string // YYYY-MM-DD
	ActivityType string // "WORKOUT", "NUTRITION"
	RecordedAt   time.Time
}

// RecordStreakActivityResult contains the updated streak state.
type RecordStreakActivityResult struct {
	CurrentStreak int32
	LongestStreak int32
	Advanced      bool
}

// RecordStreakActivityHandler coordinates updating the user's streak in a short atomic transaction.
type RecordStreakActivityHandler struct {
	userStreakRepo repository.UserStreakRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
}

// NewRecordStreakActivityHandler constructs RecordStreakActivityHandler.
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

// Handle evaluates and persists the streak update.
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

		// If no events emitted, this was an idempotent same-day activity
		if len(events) == 0 {
			result = RecordStreakActivityResult{
				CurrentStreak: userStreak.CurrentStreak(),
				LongestStreak: userStreak.LongestStreak(),
				Advanced:      false,
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
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &result, nil
}
