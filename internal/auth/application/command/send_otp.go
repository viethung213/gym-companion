package command

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	domainEvent "github.com/viethung213/gym-companion/internal/auth/domain/event"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// SendOTPCommand contains data to request an OTP code.
type SendOTPCommand struct {
	Identifier string
	Purpose    string
}

// SendOTPResult represents the result of generating and dispatching an OTP.
type SendOTPResult struct {
	Success          bool
	Message          string
	ExpiresInSeconds int64
	OTPToken         string
}

// SendOTPHandler handles OTP generation and outbox event publishing.
type SendOTPHandler struct {
	otpRepo      repository.OTPRepository
	hasher       port.Hasher
	outboxWriter port.OutboxWriter
	txManager    port.TxManager
}

// NewSendOTPHandler creates a new SendOTPHandler.
func NewSendOTPHandler(
	otpRepo repository.OTPRepository,
	hasher port.Hasher,
	outboxWriter port.OutboxWriter,
	txManager port.TxManager,
) *SendOTPHandler {
	return &SendOTPHandler{
		otpRepo:      otpRepo,
		hasher:       hasher,
		outboxWriter: outboxWriter,
		txManager:    txManager,
	}
}

// Handle generates a secure 6-digit OTP and publishes an OTPSent event.
func (h *SendOTPHandler) Handle(ctx context.Context, cmd SendOTPCommand) (*SendOTPResult, error) {
	if cmd.Identifier == "" {
		return nil, fmt.Errorf("identifier cannot be empty")
	}
	if cmd.Purpose == "" {
		return nil, fmt.Errorf("purpose cannot be empty")
	}

	// Normalize identifier if domestic or international phone
	normalizedIdentifier := cmd.Identifier
	if phone, err := vo.NewPhone(cmd.Identifier); err == nil {
		normalizedIdentifier = phone.Value()
	}

	now := time.Now()

	// 1. Kiểm tra OTP đang active gần nhất để đảm bảo cooldown chống spam
	activeOTP, err := h.otpRepo.FindLatestActive(ctx, normalizedIdentifier, cmd.Purpose)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing otp: %w", err)
	}
	if activeOTP != nil && !activeOTP.CanResend(now) {
		return nil, derror.ErrOTPResendCooldown
	}

	// 2. Sinh và băm mã OTP 6 số bảo mật
	code, hashedCode, err := h.generateSecureNumericCode(6)
	if err != nil {
		return nil, fmt.Errorf("generate secure otp: %w", err)
	}

	otpID := uuid.New().String()
	ttl := 3 * time.Minute
	cooldown := 60 * time.Second
	expiresInSeconds := int64(ttl.Seconds())

	otpEntity := entity.NewOTP(
		otpID,
		normalizedIdentifier,
		hashedCode,
		cmd.Purpose,
		ttl,
		cooldown,
		5,
	)

	// 3. Lưu OTP và Domain Event vào Outbox trong cùng một Transaction
	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Vô hiệu hóa OTP active trước đó khi và chỉ khi thời gian cooldown đã đạt chuẩn
		if activeOTP != nil {
			if err := activeOTP.Invalidate(now); err != nil {
				return err
			}
			if err := h.otpRepo.Update(txCtx, activeOTP); err != nil {
				return fmt.Errorf("invalidate previous active otp: %w", err)
			}
		}

		if err := h.otpRepo.Save(txCtx, otpEntity); err != nil {
			return fmt.Errorf("save otp: %w", err)
		}

		otpEvent := domainEvent.OTPSentEvent{
			Identifier:       normalizedIdentifier,
			Code:             code,
			ExpiresInSeconds: expiresInSeconds,
			SentAt:           now,
		}

		if err := h.outboxWriter.Write(txCtx, otpEvent); err != nil {
			return fmt.Errorf("write otp outbox event: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return &SendOTPResult{
		Success:          true,
		Message:          "Mã OTP đã được gửi thành công",
		ExpiresInSeconds: expiresInSeconds,
		OTPToken:         otpID,
	}, nil
}

// generateSecureNumericCode generates an n-digit numeric string using crypto/rand and hashes it using port.Hasher.
func (h *SendOTPHandler) generateSecureNumericCode(digits int) (string, string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", "", fmt.Errorf("generate random number: %w", err)
	}
	format := fmt.Sprintf("%%0%dd", digits)
	code := fmt.Sprintf(format, n.Int64())

	hashedCode, err := h.hasher.Hash(code)
	if err != nil {
		return "", "", fmt.Errorf("hash otp code: %w", err)
	}

	return code, hashedCode, nil
}
