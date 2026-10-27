package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

func TestSoloTrackingRoom_RoomFull_BlocksThirdParticipant(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	ruleProv := &mockRuleProvider{rule: rule}
	liveSync := &mockLiveSyncPort{}
	svc := NewSoloTrackingService(ruleProv, service.NewDefaultRuleEvaluator(), liveSync, nil, nil, nil)

	// User 1 joins -> success
	_, _, err := svc.InitializeSession(ctx, "match-full", "user-1", "plank_01", vo.ChallengeTypeDuration, 600, "plan-full-1")
	if err != nil {
		t.Fatalf("user-1 should join successfully: %v", err)
	}

	// User 2 joins -> success
	_, _, err = svc.InitializeSession(ctx, "match-full", "user-2", "plank_01", vo.ChallengeTypeDuration, 600, "plan-full-2")
	if err != nil {
		t.Fatalf("user-2 should join successfully: %v", err)
	}

	// User 3 joins -> must be rejected with ErrMatchRoomFull
	_, _, err = svc.InitializeSession(ctx, "match-full", "user-3", "plank_01", vo.ChallengeTypeDuration, 600, "plan-full-3")
	if err == nil {
		t.Fatal("expected user-3 to be rejected, got nil error")
	}
	if err != apperror.ErrMatchRoomFull {
		t.Fatalf("expected ErrMatchRoomFull, got %v", err)
	}
}

func TestSoloTrackingRoom_ExerciseMismatch(t *testing.T) {
	ctx := context.Background()
	rulePlank := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		MinConfidence: 0.5,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	rulePushup := &vo.ExerciseMotionRule{
		ExerciseID:    "push_up_01",
		ChallengeType: vo.ChallengeTypeReps,
		MinConfidence: 0.5,
	}
	ruleProvider := &mockRuleProvider{
		rules: map[string]*vo.ExerciseMotionRule{
			"plank_01":   rulePlank,
			"push_up_01": rulePushup,
		},
	}
	liveSync := &mockLiveSyncPort{}
	svc := NewSoloTrackingService(ruleProvider, nil, liveSync, nil, nil, nil)

	// User 1 joins match-exercise with plank_01
	_, _, err := svc.InitializeSession(ctx, "match-exercise", "user-1", "plank_01", vo.ChallengeTypeDuration, 600, "plan-ex-1")
	if err != nil {
		t.Fatalf("user-1 should join successfully: %v", err)
	}

	// User 2 attempts to join same match with push_up_01 -> must be rejected
	_, _, err = svc.InitializeSession(ctx, "match-exercise", "user-2", "push_up_01", vo.ChallengeTypeReps, 600, "plan-ex-2")
	if err == nil {
		t.Fatal("expected user-2 to be rejected with exercise mismatch, got nil error")
	}
	if err != apperror.ErrMatchExerciseMismatch {
		t.Fatalf("expected ErrMatchExerciseMismatch, got %v", err)
	}
}

func TestSoloTrackingRoom_WaitingOpponent_DoesNotCountReps(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		MinConfidence: 0.5,
		Rules: []vo.RuleCriterion{
			{
				Name:    "spine",
				Formula: vo.FormulaAngle3D,
				Joints:  []int{11, 23, 27},
				ValidRange: vo.ValidAngleRange{
					Min: 165.0,
					Max: 195.0,
				},
			},
		},
	}
	ruleProv := &mockRuleProvider{rule: rule}
	liveSync := &mockLiveSyncPort{} // isReady=false (only 1 user)
	svc := NewSoloTrackingService(ruleProv, service.NewDefaultRuleEvaluator(), liveSync, nil, nil, nil)

	referee, r, err := svc.InitializeSession(ctx, "match-waiting", "user-1", "plank_01", vo.ChallengeTypeDuration, 600, "plan-wait-1")
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	if !referee.IsWaiting() {
		t.Fatalf("expected referee to be in WAITING state when opponent has not joined")
	}

	// Send valid frame while WAITING
	landmarks := []vo.SkeletonPoint{
		{Index: 11, X: -1, Y: 0, Z: 0, Visibility: 0.9},
		{Index: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
		{Index: 27, X: 1, Y: 0, Z: 0, Visibility: 0.9},
	}
	res, err := svc.ProcessFrame(ctx, referee, r, landmarks, 2*time.Second, "match-waiting", "user-1", "plank_01", "plan-1", "side")
	if err != nil {
		t.Fatalf("process frame error: %v", err)
	}

	if res.MyStatus != "WAITING" {
		t.Errorf("expected MyStatus WAITING, got %s", res.MyStatus)
	}
	// Must NOT accumulate confirmed value while WAITING
	if res.MyConfirmedValue != 0 {
		t.Errorf("expected 0 confirmed value while waiting, got %d", res.MyConfirmedValue)
	}
	if res.AudioCue != "Đang chờ đối thủ vào phòng..." {
		t.Errorf("expected waiting audio cue, got %s", res.AudioCue)
	}

	// Now opponent joins
	_ = liveSync.RegisterParticipant(ctx, "match-waiting", "user-2", "plank_01")

	// Next frame should activate
	res2, err := svc.ProcessFrame(ctx, referee, r, landmarks, 1*time.Second, "match-waiting", "user-1", "plank_01", "plan-1", "side")
	if err != nil {
		t.Fatalf("process frame error after opponent joined: %v", err)
	}

	if res2.MyStatus != "ACTIVE" {
		t.Errorf("expected MyStatus ACTIVE, got %s", res2.MyStatus)
	}
	if res2.MyConfirmedValue != 1 {
		t.Errorf("expected 1 second confirmed after active, got %d", res2.MyConfirmedValue)
	}
}

func TestSoloTrackingRoom_EmitRoomReadyEvent_Idempotency(t *testing.T) {
	ctx := context.Background()
	outbox := &mockOutboxWriter{}
	svc := NewSoloTrackingService(nil, nil, nil, outbox, nil, nil)

	// First emit
	svc.emitRoomReadyEvent(ctx, "match-idempotent-1")
	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 event in outbox, got %d", len(outbox.events))
	}
	ev, ok := outbox.events[0].(*domainEvent.SoloMatchRoomReady)
	if !ok {
		t.Fatalf("expected *domainEvent.SoloMatchRoomReady, got %T", outbox.events[0])
	}
	if ev.MatchID != "match-idempotent-1" {
		t.Errorf("expected MatchID match-idempotent-1, got %s", ev.MatchID)
	}

	// Second emit for same matchID should be ignored due to sync.Map deduplication
	svc.emitRoomReadyEvent(ctx, "match-idempotent-1")
	if len(outbox.events) != 1 {
		t.Errorf("expected outbox events count to remain 1, got %d", len(outbox.events))
	}
}
