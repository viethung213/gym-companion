package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
)

// OTPRepository defines the persistence port for the OTP entity.
type OTPRepository interface {
	Save(ctx context.Context, otp *entity.OTP) error
	FindByID(ctx context.Context, id string) (*entity.OTP, error)
	FindLatestActive(ctx context.Context, identifier string, purpose string) (*entity.OTP, error)
	Update(ctx context.Context, otp *entity.OTP) error
}
