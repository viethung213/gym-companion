package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OutboxRepository implements port.OutboxRepository using GORM over PostgreSQL.
type OutboxRepository struct {
	db *gorm.DB
}

// Compile-time interface verification
var _ port.OutboxRepository = (*OutboxRepository)(nil)

// NewOutboxRepository creates a new instance of OutboxRepository.
func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) getDB(ctx context.Context) *gorm.DB {
	return GetDB(ctx, r.db)
}

// Save persists an OutboxRecord into the auth.outbox table.
func (r *OutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	if record == nil {
		return fmt.Errorf("nil outbox record")
	}
	id := record.ID
	if id == "" {
		id = uuid.New().String()
	}
	eventID := record.EventID
	if eventID == "" {
		eventID = uuid.New().String()
	}
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	var pubAt sql.NullTime
	if record.PublishedAt != nil {
		pubAt = sql.NullTime{Time: *record.PublishedAt, Valid: true}
	}
	dbOutbox := &OutboxModel{
		ID:            id,
		EventID:       eventID,
		AggregateType: record.AggregateType,
		AggregateID:   record.AggregateID,
		EventType:     record.EventType,
		Payload:       record.Payload,
		PartitionKey:  record.PartitionKey,
		CreatedAt:     createdAt,
		Published:     record.Published,
		PublishedAt:   pubAt,
		Status:        "PENDING",
	}
	if record.Published {
		dbOutbox.Status = "PUBLISHED"
	}
	if err := r.getDB(ctx).Create(dbOutbox).Error; err != nil {
		return fmt.Errorf("gorm save outbox record: %w", err)
	}
	return nil
}

// SaveEvent inserts a new event log entry directly into the auth.outbox table.
func (r *OutboxRepository) SaveEvent(ctx context.Context, eventID string, eventType string, payload []byte, partitionKey string) error {
	return r.Save(ctx, &port.OutboxRecord{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      payload,
		PartitionKey: partitionKey,
		CreatedAt:    time.Now().UTC(),
	})
}

// FetchUnpublished queries all unpublished outbox entries up to the given limit.
func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	var dbOutboxes []OutboxModel
	err := r.getDB(ctx).
		Where("published = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&dbOutboxes).
		Error

	if err != nil {
		return nil, fmt.Errorf("gorm fetch unpublished outbox events: %w", err)
	}

	records := make([]*port.OutboxRecord, 0, len(dbOutboxes))
	for _, o := range dbOutboxes {
		records = append(records, o.ToRepositoryRecord())
	}
	return records, nil
}

// MarkPublished flags the given events as published with target timestamp in a single bulk update.
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	err := r.getDB(ctx).
		Model(&OutboxModel{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"published":    true,
			"published_at": time.Now(),
			"status":       "PUBLISHED",
		}).
		Error

	if err != nil {
		return fmt.Errorf("gorm mark outbox events published: %w", err)
	}
	return nil
}

// ClaimBatch claims unpublished outbox events using FOR UPDATE SKIP LOCKED,
// updates status='PROCESSING' and locked_until=NOW()+lockDuration in a short transaction,
// and returns the claimed records.
func (r *OutboxRepository) ClaimBatch(
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

	var records []*port.OutboxRecord
	now := time.Now()
	lockedUntil := now.Add(lockDuration)

	err := r.getDB(ctx).Transaction(func(tx *gorm.DB) error {
		var dbOutboxes []OutboxModel
		err := tx.
			Clauses(clause.Locking{
				Strength: "UPDATE",
				Options:  "SKIP LOCKED",
			}).
			Where("published = ? AND (status = ? OR locked_until IS NULL OR locked_until < ?)", false, "PENDING", now).
			Order("created_at ASC").
			Limit(limit).
			Find(&dbOutboxes).
			Error

		if err != nil {
			return fmt.Errorf("gorm fetch outbox for update skip locked: %w", err)
		}

		if len(dbOutboxes) == 0 {
			return nil
		}

		ids := make([]string, len(dbOutboxes))
		records = make([]*port.OutboxRecord, len(dbOutboxes))
		for i, o := range dbOutboxes {
			records[i] = o.ToRepositoryRecord()
			ids[i] = o.ID
		}

		err = tx.Model(&OutboxModel{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":       "PROCESSING",
				"locked_until": lockedUntil,
			}).
			Error

		if err != nil {
			return fmt.Errorf("gorm claim outbox events: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return records, nil
}

// ProcessBatch claims unpublished outbox events in a short transaction,
// publishes them via publishFn OUTSIDE the database transaction,
// and marks them as published upon success.
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
		return fmt.Errorf("publish outbox batch: %w", pubErr)
	}

	ids := make([]string, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}

	return r.MarkPublished(ctx, ids)
}
