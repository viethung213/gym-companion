package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	query := GetDB(ctx, r.db)
	if model.EventID != nil && *model.EventID != "" {
		query = query.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "event_id"}},
			DoNothing: true,
		})
	}
	if err := query.Create(model).Error; err != nil {
		return fmt.Errorf("insert in-app notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) BatchSave(ctx context.Context, items []*aggregate.InAppNotification) error {
	if len(items) == 0 {
		return nil
	}
	models := make([]*InAppNotificationModel, 0, len(items))
	for _, item := range items {
		m, err := ToInAppNotificationModel(item)
		if err != nil {
			return err
		}
		models = append(models, m)
	}

	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "event_id"}},
		DoNothing: true,
	}).CreateInBatches(models, 100).Error
	if err != nil {
		return fmt.Errorf("batch insert in-app notifications: %w", err)
	}
	return nil
}

func (r *NotificationRepository) SaveSystemAnnouncement(ctx context.Context, announcement *aggregate.SystemAnnouncement) error {
	if announcement == nil {
		return nil
	}
	dataBytes, err := json.Marshal(announcement.Data())
	if err != nil {
		dataBytes = []byte("{}")
	}

	model := &SystemAnnouncementModel{
		ID:        announcement.ID(),
		EventID:   announcement.EventID(),
		Title:     announcement.Title(),
		Body:      announcement.Body(),
		Data:      dataBytes,
		CreatedAt: announcement.CreatedAt(),
	}

	err = GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("insert system announcement: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkAnnouncementAsRead(ctx context.Context, announcementID, userID string) error {
	if announcementID == "" || userID == "" {
		return nil
	}
	model := &SystemAnnouncementReadModel{
		AnnouncementID: announcementID,
		UserID:         userID,
		ReadAt:         time.Now().UTC(),
	}
	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "announcement_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("mark system announcement as read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) FindByUserAndEventID(ctx context.Context, userID, eventID string) (*aggregate.InAppNotification, error) {
	if userID == "" || eventID == "" {
		return nil, nil
	}
	var model InAppNotificationModel
	err := GetDB(ctx, r.db).
		Where("user_id = ? AND event_id = ?", userID, eventID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find in-app notification by user and event ID: %w", err)
	}
	return model.ToDomain()
}

func (r *NotificationRepository) ListByUserID(ctx context.Context, userID string, limit, offset int32) ([]*aggregate.InAppNotification, int32, error) {
	db := GetDB(ctx, r.db)

	var personalTotal int64
	if err := db.Model(&InAppNotificationModel{}).Where("user_id = ?", userID).Count(&personalTotal).Error; err != nil {
		return nil, 0, fmt.Errorf("count in-app notifications: %w", err)
	}

	var personalModels []InAppNotificationModel
	err := db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(int(limit + offset)).
		Find(&personalModels).Error
	if err != nil {
		return nil, 0, fmt.Errorf("query in-app notifications: %w", err)
	}

	var announcements []SystemAnnouncementModel
	_ = db.Order("created_at DESC").Limit(int(limit + offset)).Find(&announcements)

	var announceCount int64
	_ = db.Model(&SystemAnnouncementModel{}).Count(&announceCount)

	readMap := make(map[string]bool)
	if len(announcements) > 0 {
		announcementIDs := make([]string, 0, len(announcements))
		for _, a := range announcements {
			announcementIDs = append(announcementIDs, a.ID)
		}
		var reads []SystemAnnouncementReadModel
		_ = db.Where("user_id = ? AND announcement_id IN ?", userID, announcementIDs).Find(&reads)
		for _, read := range reads {
			readMap[read.AnnouncementID] = true
		}
	}

	combined := make([]*aggregate.InAppNotification, 0, len(personalModels)+len(announcements))
	for i := range personalModels {
		item, err := personalModels[i].ToDomain()
		if err == nil {
			combined = append(combined, item)
		}
	}

	for i := range announcements {
		dataMap := make(map[string]string)
		if len(announcements[i].Data) > 0 {
			_ = json.Unmarshal(announcements[i].Data, &dataMap)
		}
		item := aggregate.ReconstituteInAppNotification(
			announcements[i].ID,
			announcements[i].EventID,
			userID,
			announcements[i].Title,
			announcements[i].Body,
			dataMap,
			readMap[announcements[i].ID],
			announcements[i].CreatedAt,
		)
		combined = append(combined, item)
	}

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].CreatedAt().After(combined[j].CreatedAt())
	})

	total := int32(personalTotal + announceCount)
	start := int(offset)
	if start > len(combined) {
		return []*aggregate.InAppNotification{}, total, nil
	}
	end := start + int(limit)
	if end > len(combined) {
		end = len(combined)
	}

	return combined[start:end], total, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	res := GetDB(ctx, r.db).Model(&InAppNotificationModel{}).
		Where("user_id = ? AND id = ?", userID, notificationID).
		Update("is_read", true)
	if res.Error != nil {
		return fmt.Errorf("mark notification as read: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		_ = r.MarkAnnouncementAsRead(ctx, notificationID, userID)
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

	// Mark all system announcements as read for this user
	var announcements []SystemAnnouncementModel
	if err := GetDB(ctx, r.db).Select("id").Find(&announcements).Error; err == nil && len(announcements) > 0 {
		reads := make([]*SystemAnnouncementReadModel, 0, len(announcements))
		now := time.Now().UTC()
		for _, a := range announcements {
			reads = append(reads, &SystemAnnouncementReadModel{
				AnnouncementID: a.ID,
				UserID:         userID,
				ReadAt:         now,
			})
		}
		_ = GetDB(ctx, r.db).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "announcement_id"}, {Name: "user_id"}},
			DoNothing: true,
		}).CreateInBatches(reads, 100).Error
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

func (r *NotificationRepository) FindUnreadReactionsByPostAndUsers(ctx context.Context, userIDs []string, feedItemID string) ([]*aggregate.InAppNotification, error) {
	if len(userIDs) == 0 || feedItemID == "" {
		return nil, nil
	}
	var models []InAppNotificationModel
	err := GetDB(ctx, r.db).
		Where("user_id IN ? AND is_read = ? AND data->>'type' = ? AND data->>'feedItemId' = ?", userIDs, false, "POST_REACTION", feedItemID).
		Order("created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("find unread reaction notifications by users: %w", err)
	}

	result := make([]*aggregate.InAppNotification, 0, len(models))
	for i := range models {
		domainItem, dErr := models[i].ToDomain()
		if dErr == nil {
			result = append(result, domainItem)
		}
	}
	return result, nil
}

func (r *NotificationRepository) UpdateNotificationContent(ctx context.Context, item *aggregate.InAppNotification) error {
	if item == nil {
		return nil
	}
	return r.BatchUpdateNotificationContent(ctx, []*aggregate.InAppNotification{item})
}

func (r *NotificationRepository) BatchUpdateNotificationContent(ctx context.Context, items []*aggregate.InAppNotification) error {
	if len(items) == 0 {
		return nil
	}
	models := make([]*InAppNotificationModel, 0, len(items))
	for _, item := range items {
		m, err := ToInAppNotificationModel(item)
		if err != nil {
			return err
		}
		models = append(models, m)
	}

	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "body", "data", "created_at"}),
	}).CreateInBatches(models, 100).Error
	if err != nil {
		return fmt.Errorf("batch update rolled-up notifications: %w", err)
	}
	return nil
}
