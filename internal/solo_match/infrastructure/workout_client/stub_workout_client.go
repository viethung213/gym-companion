package workoutclient

import (
	"context"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
)

var _ port.UserPRProvider = (*StubWorkoutClient)(nil)

// StubWorkoutClient is an in-memory stub implementation of port.UserPRProvider.
// It serves as a local adapter to the workout_execution module until the HTTP/gRPC remote client is implemented.
type StubWorkoutClient struct{}

// NewStubWorkoutClient returns a new instance of StubWorkoutClient.
func NewStubWorkoutClient() *StubWorkoutClient {
	return &StubWorkoutClient{}
}

// GetBestPR returns nil PR to trigger default bot score fallback until integrated with workout_execution service.
func (c *StubWorkoutClient) GetBestPR(_ context.Context, _, _ string) (*float64, error) {
	return nil, nil
}
