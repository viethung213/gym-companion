package exerciseclient

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

var _ port.ExerciseProvider = (*StubExerciseClient)(nil)

// StubExerciseClient is an in-memory stub implementation of port.ExerciseProvider.
// It serves as a local adapter until the HTTP/gRPC remote client to the Exercise service is implemented.
type StubExerciseClient struct{}

// NewStubExerciseClient returns a new instance of StubExerciseClient.
func NewStubExerciseClient() *StubExerciseClient {
	return &StubExerciseClient{}
}

// GetRandomChallenge returns an exercise challenge configuration.
func (c *StubExerciseClient) GetRandomChallenge(ctx context.Context) (*port.ExerciseChallenge, error) {
	return &port.ExerciseChallenge{
		ExerciseID:    "ex_pushup",
		ExerciseName:  "Push-Up",
		ChallengeType: valueobject.ChallengeTypeReps,
		Duration:      15 * time.Minute,
	}, nil
}
