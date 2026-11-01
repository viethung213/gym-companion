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
	txManager    port.TransactionManager
	outboxWriter port.OutboxWriter
}

func NewProcessPersonalRecordHandler(
	userXpRepo repository.UserXpRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *ProcessPersonalRecordHandler {
	return &ProcessPersonalRecordHandler{
		userXpRepo:   userXpRepo,
		txManager:    txManager,
		outboxWriter: outboxWriter,
	}
}

func (h *ProcessPersonalRecordHandler) HandleEvents(
	ctx context.Context,
	events []event.Event,
) ([]event.Event, error) {
	if h == nil {
		return nil, nil
	}
	var emitted []event.Event
	for _, ev := range events {
		if e, ok := ev.(*event.PersonalRecordIngested); ok {
			xpEvents, err := h.awardPRXp(ctx, ProcessPersonalRecordCommand{
				UserID:       e.UserID,
				ExerciseID:   e.ExerciseID,
				Weight:       e.Weight,
				Reps:         e.Reps,
				ActivityDate: e.ActivityDate,
				AchievedAt:   e.AchievedAt,
			})
			if err != nil {
				return nil, err
			}
			emitted = append(emitted, xpEvents...)
		}
	}
	return emitted, nil
}

//nolint:gocritic // hugeParam: command struct is an immutable value object
func (h *ProcessPersonalRecordHandler) Handle(ctx context.Context, cmd ProcessPersonalRecordCommand) error {
	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		_, err := h.awardPRXp(txCtx, cmd)
		return err
	})
}

//nolint:gocritic // hugeParam: command struct is an immutable value object
func (h *ProcessPersonalRecordHandler) awardPRXp(
	ctx context.Context,
	cmd ProcessPersonalRecordCommand,
) ([]event.Event, error) {
	userXp, err := h.userXpRepo.FindByIDForUpdate(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user xp: %w", err)
	}
	if userXp == nil {
		userXp, err = aggregate.NewUserXp(cmd.UserID)
		if err != nil {
			return nil, fmt.Errorf("create user xp: %w", err)
		}
	}

	prSessionID := uuid.NewSHA1(
		uuid.NameSpaceOID,
		[]byte(fmt.Sprintf("PR:%s:%s:%.2f:%d:%d", cmd.UserID, cmd.ExerciseID, cmd.Weight, cmd.Reps, cmd.AchievedAt.UTC().UnixNano())),
	).String()

	alreadyProcessed, err := h.userXpRepo.HasSession(ctx, cmd.UserID, prSessionID)
	if err != nil {
		return nil, fmt.Errorf("check pr session idempotency: %w", err)
	}
	if alreadyProcessed {
		return nil, nil
	}

	xpEvents, err := userXp.EarnXp(25, "PERSONAL_RECORD", cmd.AchievedAt)
	if err != nil {
		return nil, fmt.Errorf("earn pr xp: %w", err)
	}

	if err := h.userXpRepo.Save(ctx, userXp); err != nil {
		return nil, fmt.Errorf("save user xp: %w", err)
	}

	history := entity.NewXpHistoryEntry(
		uuid.NewString(),
		cmd.UserID,
		25,
		"PERSONAL_RECORD",
		&prSessionID,
		cmd.ActivityDate,
		cmd.AchievedAt,
	)
	if err := h.userXpRepo.SaveHistory(ctx, history); err != nil {
		return nil, fmt.Errorf("save xp history: %w", err)
	}

	outboxEvents := make([]any, len(xpEvents))
	for i, ev := range xpEvents {
		outboxEvents[i] = ev
	}

	if err := h.outboxWriter.WriteEvents(ctx, "user_xp", cmd.UserID, outboxEvents); err != nil {
		return nil, fmt.Errorf("write outbox events: %w", err)
	}

	return xpEvents, nil
}
