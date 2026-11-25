// Package port defines all outbound interfaces the Coaching application layer
// depends on. Concrete adapters live under infrastructure/.
package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// Clock abstracts time.Now() for deterministic tests.
type Clock interface {
	Now() time.Time
}

// IDGenerator abstracts UUID/ULID generation.
type IDGenerator interface {
	NewID() string
}

// TransactionManager runs fn inside a database transaction. The context is
// wrapped so that repositories can pick up the tx handle from it.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// TxManager is an alias kept for parity with other bounded contexts.
type TxManager = TransactionManager

// ScheduledSession is a PENDING session with the instant it starts and the
// zone its wall-clock slot is in.
type ScheduledSession struct {
	Info     roadmap.SessionPlanInfo
	StartsAt time.Time
	Location *time.Location
}

// RoadmapRepository persists the Roadmap aggregate tree.
type RoadmapRepository interface {
	Save(ctx context.Context, r *roadmap.Roadmap) error
	FindByID(ctx context.Context, roadmapID string) (*roadmap.Roadmap, error)
	FindActiveByUser(ctx context.Context, userID string) (*roadmap.Roadmap, error)
	// ListByUser lists the user's roadmaps whose status is one of statuses,
	// newest first; an empty list matches every status.
	ListByUser(ctx context.Context, userID string, statuses []roadmap.Status, limit, offset int) ([]*roadmap.Roadmap, error)
	FindSessionByID(ctx context.Context, sessionPlanID string) (*roadmap.Roadmap, error)
	// FindPendingSessionsStarting returns PENDING sessions whose start, in their
	// roadmap's zone, falls within [from, to].
	FindPendingSessionsStarting(ctx context.Context, from, to time.Time) ([]ScheduledSession, error)
}

// OutboxRecord mirrors coaching.outbox row structure.
type OutboxRecord struct {
	ID            string     `json:"id"`
	EventID       string     `json:"eventId"`
	AggregateType string     `json:"aggregateType"`
	AggregateID   string     `json:"aggregateId"`
	EventType     string     `json:"eventType"`
	Payload       []byte     `json:"payload"`
	PartitionKey  string     `json:"partitionKey"`
	Published     bool       `json:"published"`
	CreatedAt     time.Time  `json:"createdAt"`
	PublishedAt   *time.Time `json:"publishedAt,omitempty"`
}

// OutboxRepository manages rows in coaching.outbox and coaching.outbox_log.
type OutboxRepository interface {
	Save(ctx context.Context, record *OutboxRecord) error
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)
	ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*OutboxRecord, error)
	MarkPublished(ctx context.Context, ids []string) error
	ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*OutboxRecord) error) error
	// ClaimEvent takes an inbound event for processing (D9 idempotency). It
	// returns true for a new event or one whose earlier attempt never finished
	// (redelivered after a crash), false for one already processed.
	ClaimEvent(ctx context.Context, eventID, eventType string, payload []byte) (claimed bool, err error)
	// CompleteEvent marks a claimed event processed, for good.
	CompleteEvent(ctx context.Context, eventID string) error
	// ReleaseEvent drops a claim whose handling failed, so the event is retried
	// instead of being skipped as a duplicate.
	ReleaseEvent(ctx context.Context, eventID string) error
}

// OutboxWriter is a convenience port for enqueueing domain events into the outbox
// as CloudEvents envelopes, always inside the caller's transaction.
type OutboxWriter interface {
	Enqueue(ctx context.Context, partitionKey string, events ...event.Event) error
}

// BrokerPublisher publishes outbox events to Kafka.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}

// WeekReviewQueue finds the roadmaps that hold a week ready to be closed.
type WeekReviewQueue interface {
	// RoadmapIDsWithOpenWeekEndingBy returns the ACTIVE roadmaps with an
	// unreviewed week whose end_date is on or before the given calendar date.
	// Callers decide the exact cut-off in each roadmap's zone.
	RoadmapIDsWithOpenWeekEndingBy(ctx context.Context, date time.Time) ([]string, error)
}

// RoadmapRotation drafts each user's next 4-week block ahead of time and
// swaps it in when the active one ends.
type RoadmapRotation interface {
	// FindPlannedByUser returns the user's PLANNED roadmap, or
	// roadmap.ErrRoadmapNotFound.
	FindPlannedByUser(ctx context.Context, userID string) (*roadmap.Roadmap, error)
	// DeletePlannedRoadmap removes a PLANNED roadmap and everything under it.
	// A roadmap that was activated meanwhile is left untouched.
	DeletePlannedRoadmap(ctx context.Context, roadmapID string) error
	// ActiveRoadmapsEndingBy returns the ACTIVE roadmaps whose end_date is on
	// or before date. Callers decide which of them need work.
	ActiveRoadmapsEndingBy(ctx context.Context, date time.Time) ([]RoadmapRef, error)
}

// RoadmapLifecycleStore is the persistence the roadmap lifecycle needs: the
// work that is due, and rotating one block into the next.
type RoadmapLifecycleStore interface {
	WeekReviewQueue
	RoadmapRotation
}

// RoadmapRef identifies a roadmap and the athlete it belongs to.
type RoadmapRef struct {
	RoadmapID string
	UserID    string
}
