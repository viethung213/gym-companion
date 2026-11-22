package event

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/social/domain/event"
)

const defaultNotificationDedupTTL = 1 * time.Hour

type OutboxWriter struct {
	outboxRepo   port.OutboxRepository
	deduplicator port.NotificationDeduplicator
	dedupTTL     time.Duration
}

type OutboxWriterOption func(*OutboxWriter)

// WithNotificationDeduplicator configures a Redis-backed deduplicator to suppress repeated
// like/unlike or follow/unfollow notification spam within the TTL window.
func WithNotificationDeduplicator(d port.NotificationDeduplicator) OutboxWriterOption {
	return func(w *OutboxWriter) {
		w.deduplicator = d
	}
}

// WithNotificationDedupTTL overrides the default 1h deduplication window.
func WithNotificationDedupTTL(ttl time.Duration) OutboxWriterOption {
	return func(w *OutboxWriter) {
		if ttl > 0 {
			w.dedupTTL = ttl
		}
	}
}

var _ port.EventRecorder = (*OutboxWriter)(nil)

func NewOutboxWriter(outboxRepo port.OutboxRepository, opts ...OutboxWriterOption) *OutboxWriter {
	w := &OutboxWriter{
		outboxRepo: outboxRepo,
		dedupTTL:   defaultNotificationDedupTTL,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// RecordEvents saves domain events as CloudEvent outbox records in the database.
func (w *OutboxWriter) RecordEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		if err := w.recordSingleEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

// PublishEvents delegates to RecordEvents for backward compatibility.
func (w *OutboxWriter) PublishEvents(ctx context.Context, events []any) error {
	return w.RecordEvents(ctx, events)
}

type UserFollowedEvent = domainEvent.UserFollowedEvent
type PostCreatedEvent = domainEvent.PostCreatedEvent
type PostReactedEvent = domainEvent.PostReactedEvent
type PostCommentedEvent = domainEvent.PostCommentedEvent

func (w *OutboxWriter) recordSingleEvent(ctx context.Context, ev any) error {
	if w.deduplicator != nil {
		if dedupKey := notificationDedupKey(ev); dedupKey != "" {
			allowed, _ := w.deduplicator.Allow(ctx, dedupKey, w.dedupTTL)
			if !allowed {
				slog.InfoContext(ctx, "Social notification suppressed by Redis dedup", slog.String("dedup_key", dedupKey), slog.Duration("ttl", w.dedupTTL))
				return nil
			}
		}
	}

	records, err := PackNotifications(ev)
	if err != nil {
		return err
	}

	for i := range records {
		rec := &records[i]
		outboxRec := &port.OutboxRecord{
			ID:            uuid.New().String(),
			EventID:       rec.EventID,
			AggregateType: rec.AggregateType,
			AggregateID:   rec.AggregateID,
			EventType:     rec.EventType,
			Payload:       rec.Payload,
			PartitionKey:  rec.PartitionKey,
			CreatedAt:     rec.OccurredAt,
			Published:     false,
		}

		if err := w.outboxRepo.Save(ctx, outboxRec); err != nil {
			return fmt.Errorf("save outbox record: %w", err)
		}
	}

	return nil
}

func notificationDedupKey(ev any) string {
	switch e := ev.(type) {
	case *domainEvent.PostReactedEvent:
		if e.UserID != "" && e.FeedItemID != "" && e.UserID != e.PostOwnerID {
			return fmt.Sprintf("notif:dedup:react:%s:%s", e.UserID, e.FeedItemID)
		}
	case *domainEvent.UserFollowedEvent:
		if e.FollowerID != "" && e.FollowingID != "" && e.FollowerID != e.FollowingID {
			return fmt.Sprintf("notif:dedup:follow:%s:%s", e.FollowerID, e.FollowingID)
		}
	}
	return ""
}
