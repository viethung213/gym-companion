package persistence

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"gorm.io/gorm"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type RandomIDGenerator struct{}

func (RandomIDGenerator) NewID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("read random id bytes: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:32], nil
}

type transactionKey struct{}

func (r *PostgresRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(transactionKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db.WithContext(ctx)
}

func (r *PostgresRepository) FetchUnpublished(
	ctx context.Context,
	limit int,
) ([]*port.OutboxRecord, error) {
	var rows []outboxRecord
	if err := r.getDB(ctx).
		Where("published = ?", false).
		Order("event_time, id").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("fetch coaching outbox events: %w", err)
	}
	records := make([]*port.OutboxRecord, 0, len(rows))
	for _, row := range rows {
		payload, err := json.Marshal(map[string]any{
			"specversion": "1.0",
			"id":          row.ID,
			"type":        row.EventType,
			"source":      row.Source,
			"subject":     row.Subject,
			"time":        row.EventTime,
			"data":        json.RawMessage(row.Data),
		})
		if err != nil {
			return nil, fmt.Errorf("marshal coaching cloud event: %w", err)
		}
		records = append(records, &port.OutboxRecord{
			ID:           row.ID,
			EventType:    row.EventType,
			Payload:      payload,
			PartitionKey: row.PartitionKey,
		})
	}
	return records, nil
}

func (r *PostgresRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := r.getDB(ctx).
		Model(&outboxRecord{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"published": true, "published_at": time.Now().UTC()}).Error; err != nil {
		return fmt.Errorf("mark coaching outbox events published: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ExecuteInLock(
	ctx context.Context,
	lockID int64,
	action func(context.Context) error,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acquired bool
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(?)", lockID).Scan(&acquired).Error; err != nil {
			return fmt.Errorf("acquire coaching outbox lock: %w", err)
		}
		if !acquired {
			return nil
		}
		return action(context.WithValue(ctx, transactionKey{}, tx))
	})
}

var (
	_ port.Clock            = SystemClock{}
	_ port.IDGenerator      = RandomIDGenerator{}
	_ port.OutboxRepository = (*PostgresRepository)(nil)
)
