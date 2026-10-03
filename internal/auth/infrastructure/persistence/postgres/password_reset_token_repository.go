package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	domainRepo "github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"gorm.io/gorm"
)

// PasswordResetTokenRepository implements domainRepo.PasswordResetTokenRepository using PostgreSQL and GORM.
type PasswordResetTokenRepository struct {
	db *gorm.DB
}

var _ domainRepo.PasswordResetTokenRepository = (*PasswordResetTokenRepository)(nil)

// NewPasswordResetTokenRepository creates a new PasswordResetTokenRepository instance.
func NewPasswordResetTokenRepository(db *gorm.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{db: db}
}

func toPasswordResetTokenModel(t *entity.PasswordResetToken) *PasswordResetTokenModel {
	return &PasswordResetTokenModel{
		Token:     t.Token(),
		UserID:    t.UserID(),
		ExpiresAt: t.ExpiresAt(),
		IsUsed:    t.IsUsed(),
		CreatedAt: t.CreatedAt(),
	}
}

func (m *PasswordResetTokenModel) ToDomain() *entity.PasswordResetToken {
	return entity.ReconstitutePasswordResetToken(
		m.Token,
		m.UserID,
		m.ExpiresAt,
		m.IsUsed,
		m.CreatedAt,
	)
}

// Save inserts a new password reset token.
func (r *PasswordResetTokenRepository) Save(ctx context.Context, token *entity.PasswordResetToken) error {
	m := toPasswordResetTokenModel(token)
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return fmt.Errorf("failed to save password reset token: %w", err)
	}
	return nil
}

// FindByToken retrieves a reset token record by its token string.
func (r *PasswordResetTokenRepository) FindByToken(ctx context.Context, token string) (*entity.PasswordResetToken, error) {
	var m PasswordResetTokenModel
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find password reset token: %w", err)
	}
	return m.ToDomain(), nil
}

// Update updates an existing reset token (marking it as used).
func (r *PasswordResetTokenRepository) Update(ctx context.Context, token *entity.PasswordResetToken) error {
	m := toPasswordResetTokenModel(token)
	err := r.db.WithContext(ctx).
		Model(&PasswordResetTokenModel{}).
		Where("token = ?", m.Token).
		Update("is_used", m.IsUsed).Error
	if err != nil {
		return fmt.Errorf("failed to update password reset token: %w", err)
	}
	return nil
}
