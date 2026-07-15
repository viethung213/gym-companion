package port

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type SearchFilters struct {
	BodyPartID         string
	EquipmentID        string
	TargetMuscleID     string
	SecondaryMuscleIDs []string
	TagIDs             []string
	Keyword            string
	Difficulty         string
	Limit              int32
	Offset             int32
}

type Metadata struct {
	BodyParts  []BodyPart
	Equipments []Equipment
	Muscles    []Muscle
	Tags       []Tag
}

type BodyPart struct {
	ID   string
	Name string
}

type Equipment struct {
	ID   string
	Name string
}

type Muscle struct {
	ID         string
	Name       string
	BodyPartID string
}

type Tag struct {
	ID   string
	Name string
}

type Repository interface {
	Save(ctx context.Context, exercise *domain.Exercise, event *domain.Event) error
	FindByID(ctx context.Context, id string) (*domain.Exercise, error)
	SearchActive(ctx context.Context, filters *SearchFilters) ([]*domain.Exercise, error)
	GetMetadata(ctx context.Context) (Metadata, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() (string, error)
}

// OutboxRecord represents a database outbox entry.
type OutboxRecord struct {
	ID           string
	EventID      string
	EventType    string
	Payload      []byte
	PartitionKey string
}

// OutboxRepository defines the persistence port for the outbox pattern.
type OutboxRepository interface {
	FetchUnpublished(ctx context.Context, limit int) ([]*OutboxRecord, error)
	MarkPublished(ctx context.Context, ids []string) error
	ExecuteInLock(ctx context.Context, lockID int64, fn func(ctx context.Context) error) error
}

// BrokerPublisher is the port for publishing outbox events to a broker.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}
