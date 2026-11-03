package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ port.OutboxRepository = (*OutboxRepository)(nil)

type OutboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
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

	status := "PENDING"
	if record.Published {
		status = "PUBLISHED"
	}
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	model := &OutboxModel{
		ID:            id,
		EventID:       eventID,
		AggregateType: record.AggregateType,
		AggregateID:   record.AggregateID,
		EventType:     record.EventType,
		Payload:       record.Payload,
		PartitionKey:  record.PartitionKey,
		CreatedAt:     createdAt,
		Published:     record.Published,
		PublishedAt:   record.PublishedAt,
		Status:        status,
	}

	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("insert notification outbox: %w", err)
	}
	return nil
}

func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	var models []OutboxModel
	err := GetDB(ctx, r.db).
		Where("published = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("fetch unpublished notification outbox: %w", err)
	}

	records := make([]*port.OutboxRecord, 0, len(models))
	for i := range models {
		records = append(records, models[i].ToDomain())
	}

	return records, nil
}

func (r *OutboxRepository) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}

	var records []*port.OutboxRecord

	err := GetDB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		lockedUntil := now.Add(lockDuration)

		var models []OutboxModel
		err := tx.
			Clauses(clause.Locking{
				Strength: "UPDATE",
				Options:  "SKIP LOCKED",
			}).
			Where("published = ? AND (status = ? OR locked_until IS NULL OR locked_until < ?)", false, "PENDING", now).
			Order("created_at ASC").
			Limit(limit).
			Find(&models).Error
		if err != nil {
			// Fallback without FOR UPDATE SKIP LOCKED for SQLite in-memory unit tests
			err = tx.
				Where("published = ? AND (status = ? OR locked_until IS NULL OR locked_until < ?)", false, "PENDING", now).
				Order("created_at ASC").
				Limit(limit).
				Find(&models).Error
		}
		if err != nil {
			return fmt.Errorf("select outbox for update: %w", err)
		}

		if len(models) == 0 {
			return nil
		}

		ids := make([]string, len(models))
		records = make([]*port.OutboxRecord, len(models))
		for i := range models {
			records[i] = models[i].ToDomain()
			ids[i] = models[i].ID
		}

		err = tx.Model(&OutboxModel{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":       "PROCESSING",
				"locked_until": &lockedUntil,
			}).Error
		if err != nil {
			return fmt.Errorf("update claimed outbox status: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	now := time.Now().UTC()
	err := GetDB(ctx, r.db).Model(&OutboxModel{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"published":    true,
			"published_at": &now,
			"status":       "PUBLISHED",
		}).Error
	if err != nil {
		return fmt.Errorf("mark notification outbox published: %w", err)
	}
	return nil
}

func (r *OutboxRepository) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []*port.OutboxRecord) error,
) error {
	records, err := r.ClaimBatch(ctx, limit, 30*time.Second)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	if pubErr := publishFn(ctx, records); pubErr != nil {
		return fmt.Errorf("publish notification outbox batch: %w", pubErr)
	}

	ids := make([]string, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}

	return r.MarkPublished(ctx, ids)
}
