package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

type UpdateUserRoleCommand struct {
	UserID string
	Role   string
}

func (c UpdateUserRoleCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return derror.ErrUnauthorized
	}
	if strings.TrimSpace(c.Role) == "" {
		return errors.New("role cannot be empty")
	}
	return nil
}

type UpdateUserRoleHandler struct {
	userSnapshotRepo repository.UserSnapshotRepository
	txManager        port.TransactionManager
}

func NewUpdateUserRoleHandler(
	userSnapshotRepo repository.UserSnapshotRepository,
	txManager port.TransactionManager,
) *UpdateUserRoleHandler {
	return &UpdateUserRoleHandler{
		userSnapshotRepo: userSnapshotRepo,
		txManager:        txManager,
	}
}

func (h *UpdateUserRoleHandler) Handle(ctx context.Context, cmd UpdateUserRoleCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.userSnapshotRepo.UpdateRole(txCtx, cmd.UserID, cmd.Role); err != nil {
			return fmt.Errorf("update user role snapshot: %w", err)
		}
		return nil
	})
}
