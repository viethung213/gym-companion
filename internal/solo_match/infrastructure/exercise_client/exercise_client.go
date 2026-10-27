package exerciseclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	exercisev1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

var _ port.ExerciseProvider = (*ExerciseClient)(nil)

const defaultMatchDuration = 15 * time.Minute

// RandomExerciseCaller defines the gRPC service contract to fetch random exercises.
// Any gRPC service implementation or mock satisfying this signature can be supplied.
type RandomExerciseCaller interface {
	GetRandomExercise(
		ctx context.Context,
		req *exercisev1message.GetRandomExerciseRequest,
	) (*exercisev1message.GetRandomExerciseResponse, error)
}

// ExerciseClient integrates with the Exercise service to provide competition challenges.
type ExerciseClient struct {
	caller        RandomExerciseCaller
	matchDuration time.Duration
}

// Option customizes ExerciseClient configuration.
type Option func(*ExerciseClient)

// WithMatchDuration configures competition time duration.
func WithMatchDuration(d time.Duration) Option {
	return func(c *ExerciseClient) {
		if d > 0 {
			c.matchDuration = d
		}
	}
}

// NewExerciseClient constructs a new ExerciseClient adapter.
func NewExerciseClient(caller RandomExerciseCaller, opts ...Option) *ExerciseClient {
	c := &ExerciseClient{
		caller:        caller,
		matchDuration: defaultMatchDuration,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetRandomChallenge queries the Exercise Catalog service for a competitive challenge.
func (c *ExerciseClient) GetRandomChallenge(ctx context.Context) (*port.ExerciseChallenge, error) {
	if c.caller == nil {
		return nil, errors.New("exercise rpc caller is not configured")
	}

	req := &exercisev1message.GetRandomExerciseRequest{}
	resp, err := c.caller.GetRandomExercise(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("call get random exercise grpc: %w", err)
	}

	if resp == nil || resp.Exercise == nil {
		return nil, errors.New("empty exercise response from catalog service")
	}

	ex := resp.Exercise
	challengeType := valueobject.ChallengeTypeReps
	if ex.MeasurementType == exercisev1message.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION {
		challengeType = valueobject.ChallengeTypeDuration
	}

	return &port.ExerciseChallenge{
		ExerciseID:    ex.Id,
		ExerciseName:  ex.Name,
		ChallengeType: challengeType,
		Duration:      c.matchDuration,
	}, nil
}
