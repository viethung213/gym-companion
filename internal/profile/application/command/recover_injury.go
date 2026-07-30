package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
)

type RecoverInjuryCommand struct {
	UserID   string
	InjuryID string
}

type RecoverInjuryHandler struct {
	repo      repository.UserProfileRepository
	eventPub  port.EventPublisher
	txManager port.TransactionManager
}

func NewRecoverInjuryHandler(
	repo repository.UserProfileRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *RecoverInjuryHandler {
	return &RecoverInjuryHandler{
		repo:      repo,
		eventPub:  eventPub,
		txManager: txManager,
	}
}

func (h *RecoverInjuryHandler) Handle(ctx context.Context, cmd RecoverInjuryCommand) error {
	profile, err := h.repo.FindByUserID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("find profile to recover injury: %w", err)
	}

	if err := profile.RecoverInjury(cmd.InjuryID, time.Now()); err != nil {
		return fmt.Errorf("recover injury: %w", err)
	}

	events := profile.PopEvents()

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Update(txCtx, profile); err != nil {
			return err
		}
		if h.eventPub != nil && len(events) > 0 {
			if err := h.eventPub.PublishEvents(txCtx, events); err != nil {
				return fmt.Errorf("publish injury recovered events: %w", err)
			}
		}
		return nil
	})
}
