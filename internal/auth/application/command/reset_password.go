package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// ResetPasswordCommand contains parameters for password reset using a reset token.
type ResetPasswordCommand struct {
	ResetToken      string
	NewPassword     string
	ConfirmPassword string
}

// ResetPasswordResult contains result of reset password operation.
type ResetPasswordResult struct {
	Success bool
	Message string
}

// ResetPasswordHandler handles password reset after OTP verification.
type ResetPasswordHandler struct {
	resetTokenRepo repository.PasswordResetTokenRepository
	userRepo       repository.UserRepository
	hasher         port.Hasher
	txManager      port.TxManager
}

// NewResetPasswordHandler creates a new ResetPasswordHandler.
func NewResetPasswordHandler(
	resetTokenRepo repository.PasswordResetTokenRepository,
	userRepo repository.UserRepository,
	hasher port.Hasher,
	txManager port.TxManager,
) *ResetPasswordHandler {
	return &ResetPasswordHandler{
		resetTokenRepo: resetTokenRepo,
		userRepo:       userRepo,
		hasher:         hasher,
		txManager:      txManager,
	}
}

// Handle executes the password reset using a verified reset token.
func (h *ResetPasswordHandler) Handle(ctx context.Context, cmd ResetPasswordCommand) (*ResetPasswordResult, error) {
	if cmd.ResetToken == "" {
		return nil, fmt.Errorf("reset_token cannot be empty")
	}

	newPasswordVO, err := vo.NewConfirmedRawPassword(cmd.NewPassword, cmd.ConfirmPassword)
	if err != nil {
		return nil, err
	}

	tokenEntity, err := h.resetTokenRepo.FindByToken(ctx, cmd.ResetToken)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired reset token: %w", err)
	}

	if err := tokenEntity.Use(time.Now()); err != nil {
		return nil, err
	}

	user, err := h.userRepo.FindByID(ctx, tokenEntity.UserID())
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	hashedStr, err := h.hasher.Hash(newPasswordVO.Value())
	if err != nil {
		return nil, fmt.Errorf("hash new password: %w", err)
	}
	hashedVO, err := vo.NewHashedPassword(hashedStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hashed password: %w", err)
	}

	user.UpdatePassword(hashedVO.Value())

	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.resetTokenRepo.Update(txCtx, tokenEntity); err != nil {
			return fmt.Errorf("mark token used: %w", err)
		}
		if err := h.userRepo.Update(txCtx, user); err != nil {
			return fmt.Errorf("update user password: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return &ResetPasswordResult{
		Success: true,
		Message: "Đặt lại mật khẩu thành công",
	}, nil
}
