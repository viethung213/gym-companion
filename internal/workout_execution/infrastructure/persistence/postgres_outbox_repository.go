package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostgresOutboxRepository implements port.OutboxRepository.
type PostgresOutboxRepository struct {
	db *gorm.DB
}

var _ port.OutboxRepository = (*PostgresOutboxRepository)(nil)

// NewPostgresOutboxRepository constructs repository.
func NewPostgresOutboxRepository(db *gorm.DB) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{db: db}
}

func (r *PostgresOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	db := GetDB(ctx, r.db)
	eventID := record.EventID
	if eventID == "" {
		eventID = record.ID
	}
	partitionKey := record.PartitionKey
	if partitionKey == "" {
		partitionKey = record.AggregateID
	}
	var pubAt sql.NullTime
	if record.PublishedAt != nil {
		pubAt = sql.NullTime{Time: *record.PublishedAt, Valid: true}
	}
	model := OutboxModel{
		ID:            record.ID,
		EventID:       eventID,
		AggregateType: record.AggregateType,
		AggregateID:   record.AggregateID,
		EventType:     record.EventType,
		Payload:       record.Payload,
		PartitionKey:  partitionKey,
		Published:     record.Published,
		PublishedAt:   pubAt,
		CreatedAt:     record.CreatedAt,
	}
	return db.Create(&model).Error
}

func (r *PostgresOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	db := GetDB(ctx, r.db)
	var models []OutboxModel
	err := db.Where("published = ?", false).Order("created_at ASC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unpublished outbox events: %w", err)
	}

	res := make([]*port.OutboxRecord, len(models))
	for i, m := range models {
		res[i] = OutboxToDomain(&m)
	}
	return res, nil
}

func (r *PostgresOutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	db := GetDB(ctx, r.db)
	now := time.Now().UTC()
	return db.Model(&OutboxModel{}).Where("id IN ?", ids).Updates(map[string]interface{}{
		"published":    true,
		"published_at": now,
		"status":       "PUBLISHED",
	}).Error
}

func (r *PostgresOutboxRepository) ClaimBatch(
	ctx context.Context,
	limit int,
	lockDuration time.Duration,
) ([]*port.OutboxRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}

	var res []*port.OutboxRecord
	now := time.Now().UTC()
	lockedUntil := now.Add(lockDuration)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var models []OutboxModel
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
			return fmt.Errorf("failed to fetch unpublished outbox events for update: %w", err)
		}

		if len(models) == 0 {
			return nil
		}

		res = make([]*port.OutboxRecord, len(models))
		ids := make([]string, len(models))
		for i, m := range models {
			res[i] = OutboxToDomain(&m)
			ids[i] = m.ID
		}

		err = tx.Model(&OutboxModel{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":       "PROCESSING",
				"locked_until": lockedUntil,
			}).
			Error

		if err != nil {
			return fmt.Errorf("failed to claim outbox events: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *PostgresOutboxRepository) ProcessBatch(
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
		return fmt.Errorf("publish outbox batch: %w", pubErr)
	}

	ids := make([]string, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}

	return r.MarkPublished(ctx, ids)
}
