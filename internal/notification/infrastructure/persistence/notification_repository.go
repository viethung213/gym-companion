package persistence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	"gorm.io/gorm"
)

var _ repository.NotificationRepository = (*NotificationRepository)(nil)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Save(ctx context.Context, item *aggregate.InAppNotification) error {
	dataBytes, err := json.Marshal(item.Data())
	if err != nil {
		return fmt.Errorf("marshal notification data JSON: %w", err)
	}

	query := `
		INSERT INTO notification.in_app_notifications (
			id, user_id, title, body, data, is_read, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	err = GetDB(ctx, r.db).Exec(
		query,
		item.ID(),
		item.UserID(),
		item.Title(),
		item.Body(),
		dataBytes,
		item.IsRead(),
		item.CreatedAt(),
	).Error
	if err != nil {
		return fmt.Errorf("insert in-app notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM notification.in_app_notifications
		WHERE user_id = $1
	`
	var total int32
	if err := GetDB(ctx, r.db).Raw(countQuery, userID).Row().Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count in-app notifications: %w", err)
	}

	query := `
		SELECT id, user_id, title, body, data, is_read, created_at
		FROM notification.in_app_notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := GetDB(ctx, r.db).Raw(query, userID, limit, offset).Rows()
	if err != nil {
		return nil, 0, fmt.Errorf("query in-app notifications: %w", err)
	}
	defer rows.Close()

	var items []*aggregate.InAppNotification
	for rows.Next() {
		var m InAppNotificationModel
		if err := rows.Scan(
			&m.ID,
			&m.UserID,
			&m.Title,
			&m.Body,
			&m.Data,
			&m.IsRead,
			&m.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan in-app notification row: %w", err)
		}
		item, err := m.ToDomain()
		if err != nil {
			return nil, 0, fmt.Errorf("convert in-app notification row to domain: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate in-app notification rows: %w", err)
	}

	return items, total, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	query := `
		UPDATE notification.in_app_notifications
		SET is_read = TRUE
		WHERE user_id = $1 AND id = $2
	`
	err := GetDB(ctx, r.db).Exec(query, userID, notificationID).Error
	if err != nil {
		return fmt.Errorf("mark notification as read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	query := `
		UPDATE notification.in_app_notifications
		SET is_read = TRUE
		WHERE user_id = $1 AND is_read = FALSE
	`
	err := GetDB(ctx, r.db).Exec(query, userID).Error
	if err != nil {
		return fmt.Errorf("mark all notifications as read: %w", err)
	}
	return nil
}
