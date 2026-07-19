package port

import (
	"context"
	"time"
)

// Clock abstracts time for testability.
type Clock interface {
	Now() time.Time
}

// IDGenerator abstracts ID generation for testability.
type IDGenerator interface {
	NewID() (string, error)
}

// ExerciseInfo represents exercise data returned from the Exercise module.
type ExerciseInfo struct {
	ID             string
	Name           string
	Category       string // "Compound" or "Isolation"
	PrimaryMuscle  string
	MovementPattern string
	EquipmentNeeded string
}

// ExerciseSearchFilters holds the criteria for querying exercises.
type ExerciseSearchFilters struct {
	TargetMuscleGroups []string
	AvoidInjuryAreas   []string
}

// ExerciseQueryService is the port for querying exercises from the Exercise module.
// Infrastructure implements this via gRPC adapter.
type ExerciseQueryService interface {
	SearchExercises(ctx context.Context, filters ExerciseSearchFilters) ([]ExerciseInfo, error)
}

// PlanWorkoutRequest contains inputs for the workout planner.
type PlanWorkoutRequest struct {
	AvailableExercises []ExerciseInfo
	TargetMuscleGroups []string
	Goals              []string
	ExperienceLevel    string
}

// PlanWorkoutResult contains the planner's exercise arrangement.
type PlanWorkoutResult struct {
	SelectedExerciseIDs    []string
	ReasoningExplanation   string
}

// WorkoutPlanner is the port for arranging exercises into a workout.
// Currently implemented as a Mock. Swap to Gemini SDK by replacing the implementation.
type WorkoutPlanner interface {
	PlanWorkout(ctx context.Context, req PlanWorkoutRequest) (*PlanWorkoutResult, error)
}

// OutboxRecord represents a database outbox entry for event publishing.
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

// InboxRepository defines the port for idempotent event consumption (inbox/outbox_log pattern).
type InboxRepository interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, eventID, eventType string, payload []byte, partitionKey string) error
}
