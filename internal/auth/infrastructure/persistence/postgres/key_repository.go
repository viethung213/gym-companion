package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"gorm.io/gorm"
)

// KeyRepository implements port.KeyRepository using GORM over PostgreSQL.
type KeyRepository struct {
	db *gorm.DB
}

// Compile-time interface verification
var _ port.KeyRepository = (*KeyRepository)(nil)

// NewKeyRepository creates a new instance of KeyRepository.
func NewKeyRepository(db *gorm.DB) *KeyRepository {
	return &KeyRepository{db: db}
}

func (r *KeyRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

// Save inserts a new JWK record.
func (r *KeyRepository) Save(ctx context.Context, k *port.JWKRecord) error {
	dbKey := toJSONWebKeyModel(k)
	if err := r.getDB(ctx).Create(dbKey).Error; err != nil {
		return fmt.Errorf("gorm save key: %w", err)
	}
	return nil
}

// GetActiveKey retrieves the currently active signing key.
func (r *KeyRepository) GetActiveKey(ctx context.Context) (*port.JWKRecord, error) {
	var dbKey JSONWebKeyModel
	err := r.getDB(ctx).
		Where("status = ?", port.KeyStatusActive).
		Order("created_at DESC").
		First(&dbKey).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrKeyNotFound
		}
		return nil, fmt.Errorf("gorm get active key: %w", err)
	}
	return dbKey.ToRepositoryRecord(), nil
}

// GetAllActiveAndInactiveKeys returns all keys that can be used for verification (active & inactive).
func (r *KeyRepository) GetAllActiveAndInactiveKeys(ctx context.Context) ([]*port.JWKRecord, error) {
	var dbKeys []JSONWebKeyModel
	err := r.getDB(ctx).
		Where("status IN ?", []string{port.KeyStatusActive, port.KeyStatusInactive}).
		Order("created_at DESC").
		Find(&dbKeys).
		Error

	if err != nil {
		return nil, fmt.Errorf("gorm get active and inactive keys: %w", err)
	}

	keys := make([]*port.JWKRecord, 0, len(dbKeys))
	for _, k := range dbKeys {
		keys = append(keys, k.ToRepositoryRecord())
	}
	return keys, nil
}

// UpdateStatus changes the status of a key.
func (r *KeyRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	tx := r.getDB(ctx).
		Model(&JSONWebKeyModel{}).
		Where("id = ?", id).
		Update("status", status)

	if tx.Error != nil {
		return fmt.Errorf("gorm update key status: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return apperror.ErrKeyNotFound
	}
	return nil
}

// DeleteExpiredKeys purges inactive keys whose expiration time has passed.
func (r *KeyRepository) DeleteExpiredKeys(ctx context.Context) error {
	err := r.getDB(ctx).
		Where("status = ? AND expires_at < ?", port.KeyStatusInactive, time.Now()).
		Delete(&JSONWebKeyModel{}).
		Error

	if err != nil {
		return fmt.Errorf("gorm delete expired keys: %w", err)
	}
	return nil
}
