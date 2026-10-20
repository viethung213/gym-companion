package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMOutboxRepository struct {
	db *gorm.DB
}

var _ port.OutboxRepository = (*GORMOutboxRepository)(nil)

func NewGORMOutboxRepository(db *gorm.DB) *GORMOutboxRepository {
	return &GORMOutboxRepository{db: db}
}

// Save (Application Port) lưu 1 bản ghi OutboxRecord
func (r *GORMOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	if record == nil {
		return errors.New("nil outbox record")
	}

	id := record.ID
	if id == "" {
		id = uuid.New().String()
	}
	eventID := record.EventID
	if eventID == "" {
		eventID = uuid.New().String()
	}

	m := OutboxModel{
		ID:           id,
		EventID:      eventID,
		EventType:    record.EventType,
		Payload:      record.Payload,
		PartitionKey: record.PartitionKey,
		CreatedAt:    time.Now().UTC(),
		Published:    false,
		Status:       "PENDING",
	}

	db := GetDB(ctx, r.db)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(&m).Error
	if err != nil {
		return fmt.Errorf("create outbox record: %w", err)
	}
	return nil
}

// SaveBatch lưu hàng loạt OutboxRecord
func (r *GORMOutboxRepository) SaveBatch(ctx context.Context, records []*port.OutboxRecord) error {
	if len(records) == 0 {
		return nil
	}

	now := time.Now().UTC()
	models := make([]OutboxModel, 0, len(records))
	for _, record := range records {
		id := record.ID
		if id == "" {
			id = uuid.New().String()
		}
		eventID := record.EventID
		if eventID == "" {
			eventID = uuid.New().String()
		}

		models = append(models, OutboxModel{
			ID:           id,
			EventID:      eventID,
			EventType:    record.EventType,
			Payload:      record.Payload,
			PartitionKey: record.PartitionKey,
			CreatedAt:    now,
			Published:    false,
			Status:       "PENDING",
		})
	}

	db := GetDB(ctx, r.db)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(&models).Error
	if err != nil {
		return fmt.Errorf("create outbox records batch: %w", err)
	}
	return nil
}

// FetchUnpublished lấy danh sách sự kiện chưa publish
func (r *GORMOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	db := GetDB(ctx, r.db)
	var models []OutboxModel
	if err := db.Where("published = ?", false).Order("created_at ASC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("fetch unpublished outbox events: %w", err)
	}

	records := make([]*port.OutboxRecord, 0, len(models))
	for i := range models {
		m := &models[i]
		records = append(records, &port.OutboxRecord{
			ID:           m.ID,
			EventID:      m.EventID,
			EventType:    m.EventType,
			Payload:      m.Payload,
			PartitionKey: m.PartitionKey,
		})
	}
	return records, nil
}

// ClaimBatch khóa và lấy một batch sự kiện bằng cơ chế FOR UPDATE SKIP LOCKED
// Đảm bảo không xảy ra lock contention khi nhiều worker chạy song song
func (r *GORMOutboxRepository) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}

	db := GetDB(ctx, r.db)
	now := time.Now().UTC()
	lockedUntil := now.Add(lockDuration)

	var claimedModels []OutboxModel
	err := db.Transaction(func(tx *gorm.DB) error {
		var models []OutboxModel

		// SELECT ... FOR UPDATE SKIP LOCKED:
		// Khóa các dòng được chọn, worker khác sẽ BỎ QUA (skip) các dòng này để xử lý các dòng khác song song.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("published = ? AND (status = 'PENDING' OR locked_until IS NULL OR locked_until < ?)", false, now).
			Order("created_at ASC").
			Limit(limit).
			Find(&models).Error
		if err != nil {
			return err
		}

		if len(models) == 0 {
			return nil
		}

		ids := make([]string, len(models))
		for i := range models {
			ids[i] = models[i].ID
		}

		// Đánh dấu trạng thái PROCESSING và gia hạn locked_until
		if err := tx.Model(&OutboxModel{}).Where("id IN ?", ids).
			Updates(map[string]any{
				"status":       "PROCESSING",
				"locked_until": lockedUntil,
			}).Error; err != nil {
			return err
		}

		claimedModels = models
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("claim batch outbox events: %w", err)
	}

	records := make([]*port.OutboxRecord, 0, len(claimedModels))
	for i := range claimedModels {
		m := &claimedModels[i]
		records = append(records, &port.OutboxRecord{
			ID:           m.ID,
			EventID:      m.EventID,
			EventType:    m.EventType,
			Payload:      m.Payload,
			PartitionKey: m.PartitionKey,
		})
	}
	return records, nil
}

// MarkAsPublished đánh dấu đã publish thành công
func (r *GORMOutboxRepository) MarkAsPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	err := GetDB(ctx, r.db).Model(&OutboxModel{}).Where("id IN ?", ids).
		Updates(map[string]any{
			"published":    true,
			"published_at": &now,
			"status":       "PUBLISHED",
			"locked_until": nil,
		}).Error
	if err != nil {
		return fmt.Errorf("mark outbox as published: %w", err)
	}
	return nil
}
