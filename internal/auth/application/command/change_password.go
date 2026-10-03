package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// ChangePasswordCommand contains parameters for authenticated password change.
type ChangePasswordCommand struct {
	UserID          string
	OldPassword     string
	NewPassword     string
	ConfirmPassword string
}

// ChangePasswordResult contains result of change password operation.
type ChangePasswordResult struct {
	Success bool
	Message string
}

// ChangePasswordHandler handles password change for authenticated users.
type ChangePasswordHandler struct {
	userRepo repository.UserRepository
	hasher   port.Hasher
}

// NewChangePasswordHandler creates a new ChangePasswordHandler.
func NewChangePasswordHandler(
	userRepo repository.UserRepository,
	hasher port.Hasher,
) *ChangePasswordHandler {
	return &ChangePasswordHandler{
		userRepo: userRepo,
		hasher:   hasher,
	}
}

// Handle executes the password change verifying the current old password.
func (h *ChangePasswordHandler) Handle(ctx context.Context, cmd ChangePasswordCommand) (*ChangePasswordResult, error) {
	if cmd.UserID == "" {
		return nil, derror.ErrUnauthorized
	}

	newPasswordVO, err := vo.NewConfirmedRawPassword(cmd.NewPassword, cmd.ConfirmPassword)
	if err != nil {
		return nil, err
	}

	user, err := h.userRepo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Xác thực mật khẩu cũ
	if err := h.hasher.Compare(user.Identity().CredentialData(), cmd.OldPassword); err != nil {
		return nil, derror.ErrUnauthorized
	}

	// Băm mật khẩu mới
	newHashedPassword, err := h.hasher.Hash(newPasswordVO.Value())
	if err != nil {
		return nil, fmt.Errorf("hash new password: %w", err)
	}

	hashedVO, err := vo.NewHashedPassword(newHashedPassword)
	if err != nil {
		return nil, fmt.Errorf("invalid hashed password: %w", err)
	}

	user.UpdatePassword(hashedVO.Value())

	if err := h.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user password: %w", err)
	}

	return &ChangePasswordResult{
		Success: true,
		Message: "Đổi mật khẩu thành công",
	}, nil
}
