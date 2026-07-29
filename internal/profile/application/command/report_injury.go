package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
)

type ReportInjuryCommand struct {
	InjuryID    string
	UserID      string
	MuscleGroup string
	Severity    string
	Notes       string
}

type ReportInjuryHandler struct {
	repo      repository.UserProfileRepository
	eventPub  port.EventPublisher
	txManager port.TransactionManager
}

func NewReportInjuryHandler(
	repo repository.UserProfileRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *ReportInjuryHandler {
	return &ReportInjuryHandler{
		repo:      repo,
		eventPub:  eventPub,
		txManager: txManager,
	}
}

//nolint:gocritic // Command value object passed by value for CQRS pattern consistency
func (h *ReportInjuryHandler) Handle(ctx context.Context, cmd ReportInjuryCommand) (string, error) {
	profile, err := h.repo.FindByUserID(ctx, cmd.UserID)
	if err != nil {
		return "", fmt.Errorf("find profile to report injury: %w", err)
	}

	inj, createErr := entity.NewInjury(cmd.InjuryID, cmd.MuscleGroup, cmd.Severity, cmd.Notes, time.Now())
	if createErr != nil {
		return "", fmt.Errorf("create injury entity: %w", createErr)
	}

	if addErr := profile.AddInjury(inj); addErr != nil {
		return "", fmt.Errorf("add injury to domain profile: %w", addErr)
	}

	events := profile.PopEvents()
	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if saveErr := h.repo.Update(txCtx, profile); saveErr != nil {
			return saveErr
		}
		if h.eventPub != nil && len(events) > 0 {
			if pubErr := h.eventPub.PublishEvents(txCtx, events); pubErr != nil {
				return fmt.Errorf("publish injury event: %w", pubErr)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return inj.ID(), nil
}
