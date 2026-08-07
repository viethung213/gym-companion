package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

var _ repository.DeviceRepository = (*DeviceRepository)(nil)

type DeviceRepository struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Save(ctx context.Context, device *aggregate.Device) error {
	query := `
		INSERT INTO notification.user_devices (
			id, user_id, device_token, device_type, is_active, created_at, updated_at, last_used_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, device_token) DO UPDATE SET
			is_active = EXCLUDED.is_active,
			device_type = EXCLUDED.device_type,
			updated_at = EXCLUDED.updated_at,
			last_used_at = EXCLUDED.last_used_at
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		device.ID(),
		device.UserID(),
		device.DeviceToken(),
		device.DeviceType().String(),
		device.IsActive(),
		device.CreatedAt(),
		device.UpdatedAt(),
		device.LastUsedAt(),
	)
	if err != nil {
		return fmt.Errorf("upsert user device token: %w", err)
	}
	return nil
}

func (r *DeviceRepository) GetActiveDevicesByUserID(ctx context.Context, userID string) ([]*aggregate.Device, error) {
	query := `
		SELECT id, user_id, device_token, device_type, is_active, created_at, updated_at, last_used_at
		FROM notification.user_devices
		WHERE user_id = $1 AND is_active = TRUE
		ORDER BY last_used_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query active devices: %w", err)
	}
	defer rows.Close()

	var devices []*aggregate.Device
	for rows.Next() {
		var m DeviceModel
		if err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.DeviceToken,
			&m.DeviceType,
			&m.IsActive,
			&m.CreatedAt,
			&m.UpdatedAt,
			&m.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("scan device row: %w", err)
		}
		dev, err := m.ToDomain()
		if err != nil {
			return nil, fmt.Errorf("convert device row to domain: %w", err)
		}
		devices = append(devices, dev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate device rows: %w", err)
	}

	return devices, nil
}

func (r *DeviceRepository) DeactivateTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}

	placeholders := make([]string, len(tokens))
	args := make([]interface{}, len(tokens))
	for i, tok := range tokens {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = tok
	}

	query := fmt.Sprintf(`
		UPDATE notification.user_devices
		SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP
		WHERE device_token IN (%s)
	`, strings.Join(placeholders, ","))

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("deactivate device tokens: %w", err)
	}
	return nil
}
