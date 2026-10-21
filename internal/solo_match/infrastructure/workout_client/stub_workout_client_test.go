package workoutclient_test

import (
	"context"
	"testing"

	workoutclient "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/workout_client"
)

func TestStubWorkoutClient(t *testing.T) {
	client := workoutclient.NewStubWorkoutClient()
	ctx := context.Background()

	pr, err := client.GetBestPR(ctx, "user_1", "ex_pushup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pr != nil {
		t.Errorf("expected nil PR for stub, got %v", *pr)
	}
}
