package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// VerifyOTPCommand contains data to verify an OTP.
type VerifyOTPCommand struct {
	OTPToken string
	Code     string
}

// VerifyOTPResult contains verification status and potential reset token.
type VerifyOTPResult struct {
	Valid      bool
	ResetToken string
	Message    string
}

// VerifyOTPHandler handles OTP verification and issuance of reset tokens.
type VerifyOTPHandler struct {
	otpRepo        repository.OTPRepository
	resetTokenRepo repository.PasswordResetTokenRepository
	userRepo       repository.UserRepository
	hasher         port.Hasher
}

// NewVerifyOTPHandler creates a new VerifyOTPHandler.
func NewVerifyOTPHandler(
	otpRepo repository.OTPRepository,
	resetTokenRepo repository.PasswordResetTokenRepository,
	userRepo repository.UserRepository,
	hasher port.Hasher,
) *VerifyOTPHandler {
	return &VerifyOTPHandler{
		otpRepo:        otpRepo,
		resetTokenRepo: resetTokenRepo,
		userRepo:       userRepo,
		hasher:         hasher,
	}
}

// Handle verifies the OTP code using otp_token.
func (h *VerifyOTPHandler) Handle(ctx context.Context, cmd VerifyOTPCommand) (*VerifyOTPResult, error) {
	if cmd.OTPToken == "" {
		return nil, fmt.Errorf("otp_token cannot be empty")
	}
	if cmd.Code == "" {
		return nil, fmt.Errorf("otp code cannot be empty")
	}

	otp, err := h.otpRepo.FindByID(ctx, cmd.OTPToken)
	if err != nil {
		return nil, fmt.Errorf("find otp: %w", err)
	}

	compareFn := func(plain, hash string) bool {
		return h.hasher.Compare(hash, plain) == nil
	}

	verifyErr := otp.Verify(cmd.Code, compareFn, time.Now())
	// Cập nhật lại số lần attempts hoặc trạng thái is_used
	if updateErr := h.otpRepo.Update(ctx, otp); updateErr != nil {
		return nil, fmt.Errorf("update otp state: %w", updateErr)
	}

	if verifyErr != nil {
		return nil, verifyErr
	}

	var resetToken string
	if otp.Purpose() == "reset_password" {
		// Tìm user tương ứng với identifier
		var userFound bool
		for _, idType := range []string{"email", "phone"} {
			u, err := h.userRepo.FindByIdentity(ctx, idType, otp.Identifier())
			if err == nil && u != nil {
				resetToken = uuid.New().String()
				resetEntity := entity.NewPasswordResetToken(resetToken, u.ID(), 15*time.Minute)
				if err := h.resetTokenRepo.Save(ctx, resetEntity); err != nil {
					return nil, fmt.Errorf("save reset token: %w", err)
				}
				userFound = true
				break
			}
		}

		if !userFound {
			return nil, derror.ErrNotFound
		}
	}

	return &VerifyOTPResult{
		Valid:      true,
		ResetToken: resetToken,
		Message:    "Xác thực mã OTP thành công",
	}, nil
}
