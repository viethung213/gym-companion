package exerciseclient_test

import (
	"context"
	"errors"
	"testing"
	"time"

	exercisev1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	exerciseclient "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/exercise_client"
)

type mockExerciseCaller struct {
	resp *exercisev1message.GetRandomExerciseResponse
	err  error
}

func (m *mockExerciseCaller) GetRandomExercise(
	_ context.Context,
	_ *exercisev1message.GetRandomExerciseRequest,
) (*exercisev1message.GetRandomExerciseResponse, error) {
	return m.resp, m.err
}

func TestExerciseClient_GetRandomChallenge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		caller       exerciseclient.RandomExerciseCaller
		opts         []exerciseclient.Option
		wantErr      bool
		wantExID     string
		wantName     string
		wantType     valueobject.ChallengeType
		wantDuration time.Duration
	}{
		{
			name: "successful reps challenge mapping",
			caller: &mockExerciseCaller{
				resp: &exercisev1message.GetRandomExerciseResponse{
					Exercise: &exercisev1message.ExerciseInfo{
						Id:              "ex_pushup_1",
						Name:            "Push Up",
						MeasurementType: exercisev1message.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS,
					},
				},
			},
			wantErr:      false,
			wantExID:     "ex_pushup_1",
			wantName:     "Push Up",
			wantType:     valueobject.ChallengeTypeReps,
			wantDuration: 15 * time.Minute,
		},
		{
			name: "successful duration challenge mapping with custom duration",
			caller: &mockExerciseCaller{
				resp: &exercisev1message.GetRandomExerciseResponse{
					Exercise: &exercisev1message.ExerciseInfo{
						Id:              "ex_plank_1",
						Name:            "Plank",
						MeasurementType: exercisev1message.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION,
					},
				},
			},
			opts: []exerciseclient.Option{
				exerciseclient.WithMatchDuration(10 * time.Minute),
			},
			wantErr:      false,
			wantExID:     "ex_plank_1",
			wantName:     "Plank",
			wantType:     valueobject.ChallengeTypeDuration,
			wantDuration: 10 * time.Minute,
		},
		{
			name:    "nil api client returns error",
			caller:  nil,
			wantErr: true,
		},
		{
			name: "api error propagated",
			caller: &mockExerciseCaller{
				err: errors.New("upstream service unavailable"),
			},
			wantErr: true,
		},
		{
			name: "empty response returns error",
			caller: &mockExerciseCaller{
				resp: &exercisev1message.GetRandomExerciseResponse{
					Exercise: nil,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := exerciseclient.NewExerciseClient(tt.caller, tt.opts...)
			got, err := c.GetRandomChallenge(context.Background())

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetRandomChallenge() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got.ExerciseID != tt.wantExID {
				t.Errorf("ExerciseID got = %q, want %q", got.ExerciseID, tt.wantExID)
			}
			if got.ExerciseName != tt.wantName {
				t.Errorf("ExerciseName got = %q, want %q", got.ExerciseName, tt.wantName)
			}
			if got.ChallengeType != tt.wantType {
				t.Errorf("ChallengeType got = %v, want %v", got.ChallengeType, tt.wantType)
			}
			if got.Duration != tt.wantDuration {
				t.Errorf("Duration got = %v, want %v", got.Duration, tt.wantDuration)
			}
		})
	}
}
