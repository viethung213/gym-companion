package port

import "context"

type OutboxPublisher interface {
	PublishEvent(ctx context.Context, eventType string, aggregateID string, payload interface{}) error
}

type ExerciseProvider interface {
	GetAvailableExerciseIDs(ctx context.Context, equipment []string, activeInjuries []string) ([]string, error)
	GetBaseline1RM(ctx context.Context, userID string) (map[string]float32, error)
}
