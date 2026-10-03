package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
)

// PasswordResetTokenRepository defines the persistence port for PasswordResetToken.
type PasswordResetTokenRepository interface {
	Save(ctx context.Context, token *entity.PasswordResetToken) error
	FindByToken(ctx context.Context, token string) (*entity.PasswordResetToken, error)
	Update(ctx context.Context, token *entity.PasswordResetToken) error
}
