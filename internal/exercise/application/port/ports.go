package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

// SearchFilters narrows a catalog search. Slice filters match any listed
// value; ExcludeMuscles drops exercises whose primary or any secondary muscle
// is listed. Empty Force/Mechanic and nil pointers mean "no filter".
type SearchFilters struct {
	PrimaryMuscles     []domain.MuscleGroup
	SecondaryMuscles   []domain.MuscleGroup
	Equipment          []domain.Equipment
	Categories         []domain.Category
	Levels             []domain.Level
	Force              domain.Force
	Mechanic           domain.Mechanic
	ExcludeMuscles     []domain.MuscleGroup
	HasAISupported     *bool
	Keyword            string
	Limit              int32
	Offset             int32
	MeasurementType    string
	Status             domain.Status
	IncludeAllStatuses bool
	HasMotionSpec      *bool
	HasVoiceSpec       *bool
}

type Repository interface {
	Save(ctx context.Context, exercise *domain.Exercise, event *domain.Event) error
	FindByID(ctx context.Context, id string) (*domain.Exercise, error)
	SearchActive(ctx context.Context, filters *SearchFilters) ([]*domain.Exercise, int32, error)
	GetRandomAISupported(ctx context.Context) (*domain.Exercise, error)

	// Consumer Outbox Log atomic update
	SetAISupportedWithOutboxLog(ctx context.Context, exerciseID string, supported bool, logRecord *OutboxLogRecord) error
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() (string, error)
}

// OutboxRecord represents a database outbox entry.
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

// OutboxLogRecord represents an incoming consumer idempotency log entry.
type OutboxLogRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
	Status       string
	ErrorMessage string
}

// OutboxRepository defines the persistence port for the outbox pattern.
type OutboxRepository interface {
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)
	ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*OutboxRecord, error)
	MarkPublished(ctx context.Context, ids []string) error
	ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*OutboxRecord) error) error
}

// BrokerPublisher is the port for publishing outbox events to a broker.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}
