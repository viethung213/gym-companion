package exerciseclient_test

import (
	"context"
	"testing"
	"time"

	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	exerciseclient "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/exercise_client"
)

func TestStubExerciseClient(t *testing.T) {
	client := exerciseclient.NewStubExerciseClient()
	ctx := context.Background()

	challenge, err := client.GetRandomChallenge(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if challenge == nil {
		t.Fatal("expected challenge to be non-nil")
	}

	if challenge.ExerciseID == "" {
		t.Errorf("expected non-empty exercise ID, got empty")
	}

	if challenge.ExerciseName == "" {
		t.Errorf("expected non-empty exercise name, got empty")
	}

	if !challenge.ChallengeType.IsValid() {
		t.Errorf("expected valid challenge type, got %v", challenge.ChallengeType)
	}

	if challenge.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", challenge.Duration)
	}

	if challenge.ExerciseID != "ex_pushup" || challenge.ChallengeType != valueobject.ChallengeTypeReps || challenge.Duration != 15*time.Minute {
		t.Errorf("got unexpected default values: %+v", challenge)
	}
}
