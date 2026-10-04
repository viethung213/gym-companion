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

// OTPRepository implements domainRepo.OTPRepository using PostgreSQL and GORM.
type OTPRepository struct {
	db *gorm.DB
}

var _ domainRepo.OTPRepository = (*OTPRepository)(nil)

// NewOTPRepository creates a new OTPRepository instance.
func NewOTPRepository(db *gorm.DB) *OTPRepository {
	return &OTPRepository{db: db}
}

func toOTPModel(o *entity.OTP) *OTPModel {
	return &OTPModel{
		ID:                o.ID(),
		Identifier:        o.Identifier(),
		OTPHash:           o.OTPHash(),
		Purpose:           o.Purpose(),
		Attempts:          o.Attempts(),
		MaxAttempts:       o.MaxAttempts(),
		ExpiresAt:         o.ExpiresAt(),
		ResendAvailableAt: o.ResendAvailableAt(),
		IsUsed:            o.IsUsed(),
		CreatedAt:         o.CreatedAt(),
	}
}

func (m *OTPModel) ToDomain() *entity.OTP {
	return entity.ReconstituteOTP(
		m.ID,
		m.Identifier,
		m.OTPHash,
		m.Purpose,
		m.Attempts,
		m.MaxAttempts,
		m.ExpiresAt,
		m.ResendAvailableAt,
		m.IsUsed,
		m.CreatedAt,
	)
}

func (r *OTPRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

// Save inserts a new OTP record.
func (r *OTPRepository) Save(ctx context.Context, otp *entity.OTP) error {
	m := toOTPModel(otp)
	if err := r.getDB(ctx).Create(m).Error; err != nil {
		return fmt.Errorf("failed to save otp: %w", err)
	}
	return nil
}

// FindByID retrieves an OTP record by its ID (otp_token).
func (r *OTPRepository) FindByID(ctx context.Context, id string) (*entity.OTP, error) {
	var m OTPModel
	err := r.getDB(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find otp by id: %w", err)
	}
	return m.ToDomain(), nil
}

// FindLatestActive finds the most recent active OTP for an identifier and purpose.
func (r *OTPRepository) FindLatestActive(ctx context.Context, identifier string, purpose string) (*entity.OTP, error) {
	var m OTPModel
	err := r.getDB(ctx).
		Where("identifier = ? AND purpose = ? AND is_used = ?", identifier, purpose, false).
		Order("created_at DESC").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No active OTP found
		}
		return nil, fmt.Errorf("failed to find active otp: %w", err)
	}
	return m.ToDomain(), nil
}

// Update updates an existing OTP record.
func (r *OTPRepository) Update(ctx context.Context, otp *entity.OTP) error {
	m := toOTPModel(otp)
	err := r.getDB(ctx).
		Model(&OTPModel{}).
		Where("id = ?", m.ID).
		Updates(map[string]interface{}{
			"attempts": m.Attempts,
			"is_used":  m.IsUsed,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to update otp: %w", err)
	}
	return nil
}
