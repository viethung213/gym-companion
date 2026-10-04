package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// ApproveBrandCommand carries inputs required for an admin to approve a brand request.
type ApproveBrandCommand struct {
	AdminID   string
	RequestID string
}

// ApproveBrandHandler processes the administrative approval of a brand request.
type ApproveBrandHandler struct {
	brandRepo repository.BrandRequestRepository
	userRepo  repository.UserRepository
	txManager port.TransactionManager
}

// NewApproveBrandHandler creates a new instance of ApproveBrandHandler.
func NewApproveBrandHandler(
	brandRepo repository.BrandRequestRepository,
	userRepo repository.UserRepository,
	txManager port.TransactionManager,
) *ApproveBrandHandler {
	return &ApproveBrandHandler{
		brandRepo: brandRepo,
		userRepo:  userRepo,
		txManager: txManager,
	}
}

// Handle executes the approval of a brand request within a transactional boundary.
func (h *ApproveBrandHandler) Handle(
	ctx context.Context,
	cmd ApproveBrandCommand,
) error {
	if cmd.AdminID == "" {
		return derror.ErrUnauthorized
	}
	if cmd.RequestID == "" {
		return derror.ErrInvalidBrandRequest
	}

	brandReq, err := h.brandRepo.FindByID(ctx, cmd.RequestID)
	if err != nil {
		return fmt.Errorf("find brand request: %w", err)
	}

	now := time.Now()
	if err := brandReq.Approve(cmd.AdminID, now); err != nil {
		return err
	}

	user, err := h.userRepo.FindByID(ctx, brandReq.UserID())
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	newRole, err := vo.NewRole(vo.RoleBrand)
	if err != nil {
		return fmt.Errorf("create brand role: %w", err)
	}
	user.ChangeRole(newRole)

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.userRepo.Update(txCtx, user); err != nil {
			return fmt.Errorf("update user role: %w", err)
		}
		if err := h.brandRepo.Update(txCtx, brandReq); err != nil {
			return fmt.Errorf("update brand request: %w", err)
		}
		return nil
	})
}
