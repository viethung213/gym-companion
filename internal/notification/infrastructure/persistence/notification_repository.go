package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	"gorm.io/gorm"
)

var (
	_ repository.NotificationRepository   = (*NotificationRepository)(nil)
	_ repository.ReactionRollupRepository = (*NotificationRepository)(nil)
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Save(ctx context.Context, item *aggregate.InAppNotification) error {
	model, err := ToInAppNotificationModel(item)
	if err != nil {
		return err
	}
	if err := GetDB(ctx, r.db).Create(model).Error; err != nil {
		return fmt.Errorf("insert in-app notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error) {
	db := GetDB(ctx, r.db)
	var total int64
	if err := db.Model(&InAppNotificationModel{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count in-app notifications: %w", err)
	}

	var models []InAppNotificationModel
	err := db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("query in-app notifications: %w", err)
	}

	items := make([]*aggregate.InAppNotification, 0, len(models))
	for i := range models {
		item, err := models[i].ToDomain()
		if err != nil {
			return nil, 0, fmt.Errorf("convert in-app notification model to domain: %w", err)
		}
		items = append(items, item)
	}

	return items, int32(total), nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	err := GetDB(ctx, r.db).Model(&InAppNotificationModel{}).
		Where("user_id = ? AND id = ?", userID, notificationID).
		Update("is_read", true).Error
	if err != nil {
		return fmt.Errorf("mark notification as read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	err := GetDB(ctx, r.db).Model(&InAppNotificationModel{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true).Error
	if err != nil {
		return fmt.Errorf("mark all notifications as read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) FindUnreadReactionByPost(ctx context.Context, userID, feedItemID string) (*aggregate.InAppNotification, error) {
	var model InAppNotificationModel
	err := GetDB(ctx, r.db).
		Where("user_id = ? AND is_read = ? AND data->>'type' = ? AND data->>'feedItemId' = ?", userID, false, "POST_REACTION", feedItemID).
		Order("created_at DESC").
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find unread reaction notification: %w", err)
	}
	return model.ToDomain()
}

func (r *NotificationRepository) UpdateNotificationContent(ctx context.Context, item *aggregate.InAppNotification) error {
	model, err := ToInAppNotificationModel(item)
	if err != nil {
		return err
	}
	err = GetDB(ctx, r.db).Model(&InAppNotificationModel{}).
		Where("user_id = ? AND id = ?", model.UserID, model.ID).
		Updates(map[string]any{
			"title":      model.Title,
			"body":       model.Body,
			"data":       model.Data,
			"created_at": model.CreatedAt,
		}).Error
	if err != nil {
		return fmt.Errorf("update rolled-up notification: %w", err)
	}
	return nil
}
