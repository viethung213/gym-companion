package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
)

type UpdateIdentityCommand struct {
	UserID    string
	FullName  *string
	AvatarURL *string
}

type UpdateIdentityResult struct {
	UserID    string
	FullName  string
	AvatarURL string
}

type UpdateIdentityHandler struct {
	repo      repository.UserProfileRepository
	eventPub  port.EventPublisher
	txManager port.TransactionManager
}

func NewUpdateIdentityHandler(
	repo repository.UserProfileRepository,
	eventPub port.EventPublisher,
	txManager port.TransactionManager,
) *UpdateIdentityHandler {
	return &UpdateIdentityHandler{
		repo:      repo,
		eventPub:  eventPub,
		txManager: txManager,
	}
}

//nolint:gocritic // Command value object passed by value for CQRS pattern consistency
func (h *UpdateIdentityHandler) Handle(ctx context.Context, cmd UpdateIdentityCommand) (*UpdateIdentityResult, error) {
	profile, err := h.repo.FindByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("find profile for identity update: %w", err)
	}

	if updateErr := profile.UpdateIdentity(cmd.FullName, cmd.AvatarURL); updateErr != nil {
		return nil, fmt.Errorf("update identity: %w", updateErr)
	}

	events := profile.PopEvents()

	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if repoErr := h.repo.Update(txCtx, profile); repoErr != nil {
			return repoErr
		}
		if h.eventPub != nil && len(events) > 0 {
			if pubErr := h.eventPub.PublishEvents(txCtx, events); pubErr != nil {
				return fmt.Errorf("publish events on identity update: %w", pubErr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &UpdateIdentityResult{
		UserID:    profile.UserID(),
		FullName:  profile.FullName(),
		AvatarURL: profile.AvatarURL(),
	}, nil
}
