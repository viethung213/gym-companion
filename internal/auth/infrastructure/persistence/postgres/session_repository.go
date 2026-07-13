package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SessionRepository implements port.SessionRepository using GORM over PostgreSQL.
type SessionRepository struct {
	db *gorm.DB
}

// Compile-time interface verification
var _ port.SessionRepository = (*SessionRepository)(nil)

// NewSessionRepository creates a new instance of SessionRepository.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) getDB(ctx context.Context) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

// Save inserts or updates a session token.
func (r *SessionRepository) Save(ctx context.Context, token string, userID string, expiresAt time.Time) error {
	dbSess := &SessionModel{
		Token:     hashToken(token),
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
	err := r.getDB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"expires_at"}),
	}).Create(dbSess).Error

	if err != nil {
		return fmt.Errorf("gorm save session: %w", err)
	}
	return nil
}

// FindByToken retrieves a session by its token string.
func (r *SessionRepository) FindByToken(
	ctx context.Context,
	token string,
) (*port.SessionRecord, error) {
	var dbSess SessionModel
	err := r.getDB(ctx).First(&dbSess, "token = ?", hashToken(token)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrSessionNotFound
		}
		return nil, fmt.Errorf("gorm find session: %w", err)
	}
	return dbSess.ToRepositoryRecord(), nil
}

// Delete removes a session token.
func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	tx := r.getDB(ctx).Where("token = ?", hashToken(token)).Delete(&SessionModel{})
	if tx.Error != nil {
		return fmt.Errorf("gorm delete session: %w", tx.Error)
	}
	if tx.RowsAffected == 0 {
		return apperror.ErrSessionNotFound
	}
	return nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// DeleteAllByUserID revokes all active sessions for a user (e.g. force logout).
func (r *SessionRepository) DeleteAllByUserID(ctx context.Context, userID string) error {
	err := r.getDB(ctx).Where("user_id = ?", userID).Delete(&SessionModel{}).Error
	if err != nil {
		return fmt.Errorf("gorm delete all user sessions: %w", err)
	}
	return nil
}
