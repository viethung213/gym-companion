package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// RegisterBrandCommand carries inputs required to submit a brand registration request.
type RegisterBrandCommand struct {
	UserID       string
	BrandName    string
	Description  string
	ContactPhone string
	Address      string
}

// RegisterBrandHandler processes user requests to upgrade their account to Brand.
type RegisterBrandHandler struct {
	brandRepo repository.BrandRequestRepository
	userRepo  repository.UserRepository
}

// NewRegisterBrandHandler creates a new instance of RegisterBrandHandler.
func NewRegisterBrandHandler(
	brandRepo repository.BrandRequestRepository,
	userRepo repository.UserRepository,
) *RegisterBrandHandler {
	return &RegisterBrandHandler{
		brandRepo: brandRepo,
		userRepo:  userRepo,
	}
}

// Handle executes the brand registration request logic.
func (h *RegisterBrandHandler) Handle(
	ctx context.Context,
	cmd RegisterBrandCommand,
) (*entity.BrandRequest, error) {
	if cmd.UserID == "" {
		return nil, derror.ErrUnauthorized
	}

	user, err := h.userRepo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	if user.Role() == vo.RoleBrand {
		return nil, derror.ErrUserAlreadyBrand
	}

	// Ensure no duplicate pending request for this user
	pendingReq, err := h.brandRepo.FindPendingByUserID(ctx, cmd.UserID)
	if err == nil && pendingReq != nil {
		return nil, derror.ErrBrandRequestAlreadyPending
	}

	requestID := uuid.New().String()
	brandReq, err := entity.NewBrandRequest(
		requestID,
		cmd.UserID,
		cmd.BrandName,
		cmd.Description,
		cmd.ContactPhone,
		cmd.Address,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}

	if err := h.brandRepo.Create(ctx, brandReq); err != nil {
		return nil, fmt.Errorf("create brand request: %w", err)
	}

	return brandReq, nil
}
