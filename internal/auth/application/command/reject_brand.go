package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// RejectBrandCommand carries inputs required for an admin to reject a brand request.
type RejectBrandCommand struct {
	AdminID   string
	RequestID string
	Reason    string
}

// RejectBrandHandler processes the administrative rejection of a brand request.
type RejectBrandHandler struct {
	brandRepo repository.BrandRequestRepository
}

// NewRejectBrandHandler creates a new instance of RejectBrandHandler.
func NewRejectBrandHandler(brandRepo repository.BrandRequestRepository) *RejectBrandHandler {
	return &RejectBrandHandler{
		brandRepo: brandRepo,
	}
}

// Handle executes the rejection of a brand request.
func (h *RejectBrandHandler) Handle(
	ctx context.Context,
	cmd RejectBrandCommand,
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
	if err := brandReq.Reject(cmd.AdminID, cmd.Reason, now); err != nil {
		return err
	}

	if err := h.brandRepo.Update(ctx, brandReq); err != nil {
		return fmt.Errorf("update brand request: %w", err)
	}

	return nil
}
