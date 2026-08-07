package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

var _ repository.SettingRepository = (*PostgresSettingRepository)(nil)

type PostgresSettingRepository struct {
	db *sql.DB
}

func NewPostgresSettingRepository(db *sql.DB) *PostgresSettingRepository {
	return &PostgresSettingRepository{db: db}
}

func (r *PostgresSettingRepository) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	query := `
		SELECT user_id, enable_push, enable_email, enable_sms, quiet_hours_start, quiet_hours_end, created_at, updated_at
		FROM notification.user_settings
		WHERE user_id = $1
	`
	var m SettingModel
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&m.UserID,
		&m.EnablePush,
		&m.EnableEmail,
		&m.EnableSMS,
		&m.QuietHoursStart,
		&m.QuietHoursEnd,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, derror.ErrSettingNotFound
		}
		return nil, fmt.Errorf("query user notification setting: %w", err)
	}

	return m.ToDomain(), nil
}

func (r *PostgresSettingRepository) Save(ctx context.Context, setting *aggregate.Setting) error {
	query := `
		INSERT INTO notification.user_settings (
			user_id, enable_push, enable_email, enable_sms, quiet_hours_start, quiet_hours_end, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			enable_push = EXCLUDED.enable_push,
			enable_email = EXCLUDED.enable_email,
			enable_sms = EXCLUDED.enable_sms,
			quiet_hours_start = EXCLUDED.quiet_hours_start,
			quiet_hours_end = EXCLUDED.quiet_hours_end,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		setting.UserID(),
		setting.EnablePush(),
		setting.EnableEmail(),
		setting.EnableSMS(),
		setting.QuietHoursStart(),
		setting.QuietHoursEnd(),
		setting.CreatedAt(),
		setting.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("upsert user notification setting: %w", err)
	}
	return nil
}
