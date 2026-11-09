package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

type mockExerciseRuleProvider struct {
	rule *vo.ExerciseMotionRule
	err  error
}

func (m *mockExerciseRuleProvider) GetRule(ctx context.Context, exerciseID string) (*vo.ExerciseMotionRule, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.rule, nil
}

func TestWorkoutTrackingService_Lifecycle(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:             "push_up",
		ExerciseName:           "Push-up",
		ChallengeType:          vo.ChallengeTypeReps,
		TimeCeiling:            10 * time.Minute,
		RecommendedCameraAngle: "side",
		PhaseDetection: &vo.PhaseDetectionConfig{
			Metric: "elbow_angle",
			Thresholds: vo.PhaseThresholds{
				Rest:       vo.ThresholdCondition{GTE: 150},
				Transition: vo.ThresholdCondition{GT: 90, LT: 150},
				Active:     vo.ThresholdCondition{LTE: 90},
			},
		},
	}

	provider := &mockExerciseRuleProvider{rule: rule}
	svc := command.NewWorkoutTrackingService(provider, service.NewDefaultRuleEvaluator())

	ctx := context.Background()

	// 1. InitializeSession
	referee, motionRule, err := svc.InitializeSession(
		ctx,
		"sess-test-123",
		"user-athlete-1",
		"push_up",
		"side",
		vo.ChallengeTypeReps,
		10,
		0,
	)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}
	if referee == nil || motionRule == nil {
		t.Fatal("expected non-nil referee and motionRule")
	}

	// 2. ProcessFrame (Rest position)
	landmarks := make([]vo.SkeletonPoint, 17)
	// Right shoulder, elbow, wrist
	landmarks[6] = vo.SkeletonPoint{Index: 6, X: 0.5, Y: 0.2, Visibility: 0.9}
	landmarks[8] = vo.SkeletonPoint{Index: 8, X: 0.5, Y: 0.5, Visibility: 0.9}
	landmarks[10] = vo.SkeletonPoint{Index: 10, X: 0.5, Y: 0.8, Visibility: 0.9}

	res, err := svc.ProcessFrame(ctx, referee, motionRule, landmarks, 50*time.Millisecond, "side")
	if err != nil {
		t.Fatalf("ProcessFrame failed: %v", err)
	}
	if res.IsSetCompleted {
		t.Error("expected set not completed yet")
	}

	// 3. SubmitSet
	finalRes, err := svc.SubmitSet(ctx, referee)
	if err != nil {
		t.Fatalf("SubmitSet failed: %v", err)
	}
	if !finalRes.IsSetCompleted {
		t.Error("expected finalRes.IsSetCompleted == true after submit")
	}
}
