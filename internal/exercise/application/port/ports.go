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

	// BodyPart CRUD
	CreateBodyPart(ctx context.Context, bp *BodyPart) error
	GetBodyPart(ctx context.Context, id string) (*BodyPart, error)
	ListBodyParts(ctx context.Context, limit, offset int) ([]BodyPart, int, error)
	UpdateBodyPart(ctx context.Context, bp *BodyPart) error
	DeleteBodyPart(ctx context.Context, id string) error

	// Equipment CRUD
	CreateEquipment(ctx context.Context, eq *Equipment) error
	GetEquipment(ctx context.Context, id string) (*Equipment, error)
	ListEquipments(ctx context.Context, limit, offset int) ([]Equipment, int, error)
	UpdateEquipment(ctx context.Context, eq *Equipment) error
	DeleteEquipment(ctx context.Context, id string) error

	// Muscle CRUD
	CreateMuscle(ctx context.Context, m *Muscle) error
	GetMuscle(ctx context.Context, id string) (*Muscle, error)
	ListMuscles(ctx context.Context, bodyPartID string, limit, offset int) ([]Muscle, int, error)
	UpdateMuscle(ctx context.Context, m *Muscle) error
	DeleteMuscle(ctx context.Context, id string) error

	// Tag CRUD
	CreateTag(ctx context.Context, t *Tag) error
	GetTag(ctx context.Context, id string) (*Tag, error)
	ListTags(ctx context.Context, limit, offset int) ([]Tag, int, error)
	UpdateTag(ctx context.Context, t *Tag) error
	DeleteTag(ctx context.Context, id string) error
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
	ProcessBatch(ctx context.Context, limit int, publishFn func(ctx context.Context, records []*OutboxRecord) error) error
}

// BrokerPublisher is the port for publishing outbox events to a broker.
type BrokerPublisher interface {
	PublishBatch(ctx context.Context, records []*OutboxRecord) error
}
