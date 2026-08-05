package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresOutboxRepository struct {
	db *gorm.DB
}

var _ port.OutboxRepository = (*PostgresOutboxRepository)(nil)

func NewPostgresOutboxRepository(db *gorm.DB) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{db: db}
}

func (r *PostgresOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	gormRecord := &GormOutbox{
		ID:           record.ID,
		EventID:      record.EventID,
		EventType:    record.EventType,
		Payload:      record.Payload,
		PartitionKey: record.PartitionKey,
		CreatedAt:    time.Now(),
		Published:    false,
		Status:       "PENDING",
	}

	db := getDB(ctx, r.db)
	if err := db.Create(gormRecord).Error; err != nil {
		return fmt.Errorf("outbox repo save record: %w", err)
	}
	return nil
}

func (r *PostgresOutboxRepository) SaveEvent(ctx context.Context, eventID, eventType, partitionKey string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("outbox repo marshal payload: %w", err)
	}

	record := &port.OutboxRecord{
		ID:           eventID,
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payloadBytes,
		PartitionKey: partitionKey,
	}

	return r.Save(ctx, record)
}

func (r *PostgresOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]port.OutboxRecord, error) {
	var gormRecords []GormOutbox
	db := getDB(ctx, r.db)
	if err := db.Where("published = ?", false).Order("created_at ASC").Limit(limit).Find(&gormRecords).Error; err != nil {
		return nil, fmt.Errorf("outbox repo fetch unpublished: %w", err)
	}

	result := make([]port.OutboxRecord, 0, len(gormRecords))
	for i := range gormRecords {
		rec := &gormRecords[i]
		result = append(result, port.OutboxRecord{
			ID:           rec.ID,
			EventID:      rec.EventID,
			EventType:    rec.EventType,
			Payload:      rec.Payload,
			PartitionKey: rec.PartitionKey,
			CreatedAt:    rec.CreatedAt,
			Published:    rec.Published,
		})
	}
	return result, nil
}

func (r *PostgresOutboxRepository) ClaimBatch(
	ctx context.Context,
	limit int,
	lockDuration time.Duration,
) ([]port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}

	var records []port.OutboxRecord
	now := time.Now()
	lockedUntil := now.Add(lockDuration)

	db := getDB(ctx, r.db)
	err := db.Transaction(func(tx *gorm.DB) error {
		var models []GormOutbox
		err := tx.
			Clauses(clause.Locking{
				Strength: "UPDATE",
				Options:  "SKIP LOCKED",
			}).
			Where("published = ? AND (status = ? OR locked_until IS NULL OR locked_until < ?)", false, "PENDING", now).
			Order("created_at ASC").
			Limit(limit).
			Find(&models).
			Error

		if err != nil {
			return fmt.Errorf("fetch unpublished outbox events for update: %w", err)
		}

		if len(models) == 0 {
			return nil
		}

		records = make([]port.OutboxRecord, 0, len(models))
		ids := make([]string, 0, len(models))
		for i := range models {
			m := &models[i]
			records = append(records, port.OutboxRecord{
				ID:           m.ID,
				EventID:      m.EventID,
				EventType:    m.EventType,
				Payload:      m.Payload,
				PartitionKey: m.PartitionKey,
				CreatedAt:    m.CreatedAt,
				Published:    m.Published,
			})
			ids = append(ids, m.ID)
		}

		err = tx.Model(&GormOutbox{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":       "PROCESSING",
				"locked_until": lockedUntil,
			}).
			Error

		if err != nil {
			return fmt.Errorf("claim outbox records: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return records, nil
}

func (r *PostgresOutboxRepository) MarkAsPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	db := getDB(ctx, r.db)
	now := time.Now()
	err := db.Model(&GormOutbox{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"published":    true,
			"published_at": sql.NullTime{Time: now, Valid: true},
			"status":       "PUBLISHED",
		}).Error
	if err != nil {
		return fmt.Errorf("mark outbox records as published: %w", err)
	}
	return nil
}

func (r *PostgresOutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	return r.MarkAsPublished(ctx, []string{eventID})
}

func (r *PostgresOutboxRepository) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []port.OutboxRecord) error,
) error {
	records, err := r.ClaimBatch(ctx, limit, 30*time.Second)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	if pubErr := publishFn(ctx, records); pubErr != nil {
		return fmt.Errorf("publish outbox batch: %w", pubErr)
	}

	ids := make([]string, 0, len(records))
	for _, rec := range records {
		ids = append(ids, rec.ID)
	}

	return r.MarkAsPublished(ctx, ids)
}
