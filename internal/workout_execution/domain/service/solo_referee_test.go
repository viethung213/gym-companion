package service

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

func TestSoloRefereeSession_NilRule_ReturnsError(t *testing.T) {
	session, err := NewSoloRefereeSession("squat_01", vo.ChallengeTypeReps, 10*time.Minute, nil)
	if err == nil {
		t.Fatal("expected error when rule is nil, got nil")
	}
	if session != nil {
		t.Errorf("expected nil session on error, got %v", session)
	}
}

func TestSoloRefereeSession_Duration_EmptyRules_ReturnsError(t *testing.T) {
	rule := &vo.ExerciseMotionRule{ExerciseID: "plank_01", ChallengeType: vo.ChallengeTypeDuration}
	session, err := NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, 30*time.Minute, rule)
	if err == nil {
		t.Fatal("expected error when duration rule has no criteria, got nil")
	}
	if session != nil {
		t.Errorf("expected nil session on error, got %v", session)
	}
}

func TestSoloRefereeSession_Duration_AccumulatesTime(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	session, err := NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, 30*time.Minute, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 5 seconds of valid frames (1s per frame)
	for i := 0; i < 5; i++ {
		session.ProcessFrame(vo.EvaluationResult{Passed: true, FormScore: 100}, 1*time.Second)
	}

	if session.ConfirmedValue() != 5 {
		t.Errorf("expected 5 seconds accumulated, got %d", session.ConfirmedValue())
	}
	if session.Status() != ParticipantStatusActive {
		t.Errorf("expected status ACTIVE, got %v", session.Status())
	}
}

func TestSoloRefereeSession_Duration_ViolationBefore5s_DoesNotResetAccumulated(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	session, err := NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, 30*time.Minute, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step 1: Accumulate 10 seconds of valid plank
	for i := 0; i < 10; i++ {
		session.ProcessFrame(vo.EvaluationResult{Passed: true, FormScore: 95}, 1*time.Second)
	}
	if session.ConfirmedValue() != 10 {
		t.Fatalf("expected 10s, got %d", session.ConfirmedValue())
	}

	// Step 2: Form broken for 3 seconds (less than 5s)
	for i := 0; i < 3; i++ {
		session.ProcessFrame(vo.EvaluationResult{Passed: false, FormScore: 80}, 1*time.Second)
	}
	// Time should NOT increase during violation, but accumulated time must remain 10s!
	if session.ConfirmedValue() != 10 {
		t.Errorf("expected accumulated time to remain 10s during break, got %d", session.ConfirmedValue())
	}
	if session.IsCompleted() {
		t.Errorf("expected session not to be completed after only 3s violation")
	}

	// Step 3: Returns to valid form -> continue accumulating from 10s, NOT reset to 0!
	session.ProcessFrame(vo.EvaluationResult{Passed: true, FormScore: 95}, 1*time.Second)
	if session.ConfirmedValue() != 11 {
		t.Errorf("expected 11s after fixing form, got %d", session.ConfirmedValue())
	}
	if session.Status() != ParticipantStatusActive {
		t.Errorf("expected status ACTIVE, got %v", session.Status())
	}
}

func TestSoloRefereeSession_Duration_5sViolation_TransitionsToSpectating(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	session, err := NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, 30*time.Minute, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Accumulate 20 seconds
	for i := 0; i < 20; i++ {
		session.ProcessFrame(vo.EvaluationResult{Passed: true, FormScore: 90}, 1*time.Second)
	}

	// Break form continuously for 5 seconds
	for i := 0; i < 5; i++ {
		session.ProcessFrame(vo.EvaluationResult{Passed: false, FormScore: 70}, 1*time.Second)
	}

	if !session.IsCompleted() {
		t.Errorf("expected session to be completed after 5s continuous violation")
	}
	if session.Status() != ParticipantStatusSpectating {
		t.Errorf("expected status SPECTATING, got %v", session.Status())
	}
	if session.CompletionReason() != "FORM_VIOLATION_5S" {
		t.Errorf("expected reason FORM_VIOLATION_5S, got '%s'", session.CompletionReason())
	}
	// Final confirmed value should remain frozen at 20s
	if session.ConfirmedValue() != 20 {
		t.Errorf("expected frozen confirmed value 20, got %d", session.ConfirmedValue())
	}
}

func TestSoloRefereeSession_Reps_NilPhaseDetection_ReturnsError(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "squat_01",
		ChallengeType: vo.ChallengeTypeReps,
	}
	session, err := NewSoloRefereeSession("squat_01", vo.ChallengeTypeReps, 10*time.Minute, rule)
	if err == nil {
		t.Fatal("expected error when phase_detection is nil for rep challenge, got nil")
	}
	if session != nil {
		t.Errorf("expected nil session, got %v", session)
	}
}

func TestSoloRefereeSession_Reps_CycleIncrementsCount(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID: "squat_01",
		PhaseDetection: &vo.PhaseDetectionConfig{
			Metric: "knee_angle",
			Thresholds: vo.PhaseThresholds{
				Rest:       vo.ThresholdCondition{GT: 165.0},
				Transition: vo.ThresholdCondition{GT: 85.0, LTE: 165.0},
				Active:     vo.ThresholdCondition{LTE: 85.0},
			},
		},
	}
	session, err := NewSoloRefereeSession("squat_01", vo.ChallengeTypeReps, 10*time.Minute, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step 1: Standing at 170 deg (Rest)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 170, FormScore: 100}, 500*time.Millisecond)
	if session.ConfirmedValue() != 0 {
		t.Fatalf("expected 0 reps initially, got %d", session.ConfirmedValue())
	}

	// Step 2: Entering transition at 120 deg (Transition)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 120, FormScore: 95}, 500*time.Millisecond)
	if session.ConfirmedValue() != 0 {
		t.Fatalf("expected 0 reps in transition, got %d", session.ConfirmedValue())
	}

	// Step 3: Squatting down to 80 deg (Reaches Inflection <= 85)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 80, FormScore: 95}, 500*time.Millisecond)
	if session.ConfirmedValue() != 0 {
		t.Fatalf("expected 0 reps at inflection, got %d", session.ConfirmedValue())
	}

	// Step 4: Returning up through transition at 120 deg (Transition)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 120, FormScore: 95}, 500*time.Millisecond)
	if session.ConfirmedValue() != 0 {
		t.Fatalf("expected 0 reps in returning transition, got %d", session.ConfirmedValue())
	}

	// Step 5: Standing back up to 170 deg (Reaches Return >= 165) -> +1 completed rep!
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 170, FormScore: 95}, 500*time.Millisecond)
	if session.ConfirmedValue() != 1 {
		t.Errorf("expected 1 completed rep, got %d", session.ConfirmedValue())
	}
}

func TestSoloRefereeSession_PhaseDetection_IncrementsRepCycle(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:  "bicep_curl_01",
		DisplayName: "Bicep Curl",
		RepType:     "counted",
		PhaseDetection: &vo.PhaseDetectionConfig{
			Metric: "elbow_angle",
			Thresholds: vo.PhaseThresholds{
				Rest:       vo.ThresholdCondition{GT: 155.0},
				Transition: vo.ThresholdCondition{GT: 80.0, LTE: 155.0},
				Active:     vo.ThresholdCondition{LTE: 80.0},
			},
			Calculation: vo.PhaseCalculation{
				Joints: []int{11, 13, 15},
			},
		},
	}
	session, err := NewSoloRefereeSession("bicep_curl_01", vo.ChallengeTypeReps, 10*time.Minute, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step 1: Arm fully extended at 160 deg (Rest > 155)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 160, FormScore: 100}, 500*time.Millisecond)
	if session.ConfirmedValue() != 0 {
		t.Fatalf("expected 0 reps at rest, got %d", session.ConfirmedValue())
	}
	if session.RepPhase() != RepPhaseRest {
		t.Errorf("expected phase REST, got %v", session.RepPhase())
	}

	// Step 2: Curling into transition at 120 deg (Transition > 80 and <= 155)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 120, FormScore: 95}, 200*time.Millisecond)
	if session.ConfirmedValue() != 0 {
		t.Fatalf("expected 0 reps in transition, got %d", session.ConfirmedValue())
	}
	if session.RepPhase() != RepPhaseTransition {
		t.Errorf("expected phase TRANSITION, got %v", session.RepPhase())
	}

	// Step 3: Curling up to peak contraction at 75 deg (Active <= 80)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 75, FormScore: 95}, 200*time.Millisecond)
	if session.ConfirmedValue() != 0 {
		t.Fatalf("expected 0 reps at active, got %d", session.ConfirmedValue())
	}
	if session.RepPhase() != RepPhaseActive {
		t.Errorf("expected phase ACTIVE, got %v", session.RepPhase())
	}

	// Step 4: Lowering weight into transition at 110 deg
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 110, FormScore: 95}, 200*time.Millisecond)
	if session.ConfirmedValue() != 0 {
		t.Fatalf("expected 0 reps in lowering transition, got %d", session.ConfirmedValue())
	}
	if session.RepPhase() != RepPhaseTransition {
		t.Errorf("expected phase TRANSITION, got %v", session.RepPhase())
	}

	// Step 5: Extending fully back to rest at 160 deg (Rest > 155) -> +1 completed rep!
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 160, FormScore: 95}, 200*time.Millisecond)
	if session.ConfirmedValue() != 1 {
		t.Errorf("expected 1 completed rep after returning to rest, got %d", session.ConfirmedValue())
	}
	if session.RepPhase() != RepPhaseRest {
		t.Errorf("expected phase REST after completing rep, got %v", session.RepPhase())
	}

	// Cheat Test 1: Moving to transition (100 deg) but NOT reaching active (<= 80), then returning to rest
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 100, FormScore: 95}, 200*time.Millisecond)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 160, FormScore: 95}, 200*time.Millisecond)
	if session.ConfirmedValue() != 1 {
		t.Errorf("cheat attempt without reaching active must NOT increment rep: got %d, want 1", session.ConfirmedValue())
	}

	// Cheat Test 2: Reaching active (75 deg) but only lowering to transition (110 deg) and bouncing back to active (75 deg)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 120, FormScore: 95}, 200*time.Millisecond)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 75, FormScore: 95}, 200*time.Millisecond)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 110, FormScore: 95}, 200*time.Millisecond)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 75, FormScore: 95}, 200*time.Millisecond)
	if session.ConfirmedValue() != 1 {
		t.Errorf("bouncing in active without returning to rest must NOT increment rep: got %d, want 1", session.ConfirmedValue())
	}

	// Finally lowering fully to rest -> rep 2 completed!
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 110, FormScore: 95}, 200*time.Millisecond)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 160, FormScore: 95}, 200*time.Millisecond)
	if session.ConfirmedValue() != 2 {
		t.Errorf("expected 2 completed reps after returning to rest, got %d", session.ConfirmedValue())
	}
}

func TestSoloRefereeSession_StrictSequence_SkipTransition_DoesNotCount(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID: "squat_01",
		PhaseDetection: &vo.PhaseDetectionConfig{
			Metric: "knee_angle",
			Thresholds: vo.PhaseThresholds{
				Rest:       vo.ThresholdCondition{GT: 155.0},
				Transition: vo.ThresholdCondition{GT: 80.0, LTE: 155.0},
				Active:     vo.ThresholdCondition{LTE: 80.0},
			},
		},
	}
	session, err := NewSoloRefereeSession("squat_01", vo.ChallengeTypeReps, 10*time.Minute, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Frame 1: Rest (170 deg)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 170}, 100*time.Millisecond)

	// Frame 2: Teleports directly to Active (75 deg) without transition frame
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 75}, 100*time.Millisecond)

	// Frame 3: Controlled return transition (110 deg)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 110}, 100*time.Millisecond)

	// Frame 4: Returns to Rest (170 deg)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 170}, 100*time.Millisecond)

	// Rep must NOT count because Checkpoint 1 (hasTransitionedIn) was skipped!
	if session.ConfirmedValue() != 0 {
		t.Errorf("skipping inward transition must NOT count rep: got %d, want 0", session.ConfirmedValue())
	}
}

func TestSoloRefereeSession_StrictSequence_SkipReturnTransition_DoesNotCount(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID: "squat_01",
		PhaseDetection: &vo.PhaseDetectionConfig{
			Metric: "knee_angle",
			Thresholds: vo.PhaseThresholds{
				Rest:       vo.ThresholdCondition{GT: 155.0},
				Transition: vo.ThresholdCondition{GT: 80.0, LTE: 155.0},
				Active:     vo.ThresholdCondition{LTE: 80.0},
			},
		},
	}
	session, err := NewSoloRefereeSession("squat_01", vo.ChallengeTypeReps, 10*time.Minute, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Frame 1: Rest (170 deg)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 170}, 100*time.Millisecond)

	// Frame 2: Transition In (120 deg)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 120}, 100*time.Millisecond)

	// Frame 3: Active (75 deg)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 75}, 100*time.Millisecond)

	// Frame 4: Teleports directly from Active (75 deg) to Rest (170 deg) skipping return transition
	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 170}, 100*time.Millisecond)

	// Rep must NOT count because Checkpoint 3 (hasTransitionedOut) was skipped!
	if session.ConfirmedValue() != 0 {
		t.Errorf("skipping outward transition must NOT count rep: got %d, want 0", session.ConfirmedValue())
	}
}

func TestSoloRefereeSession_TimeoutCeiling(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "pushup_01",
		ChallengeType: vo.ChallengeTypeReps,
		PhaseDetection: &vo.PhaseDetectionConfig{
			Thresholds: vo.PhaseThresholds{
				Rest:       vo.ThresholdCondition{GT: 160.0},
				Transition: vo.ThresholdCondition{GT: 90.0, LTE: 160.0},
				Active:     vo.ThresholdCondition{LTE: 90.0},
			},
		},
	}
	session, err := NewSoloRefereeSession("pushup_01", vo.ChallengeTypeReps, 3*time.Second, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 170}, 2*time.Second)
	if session.IsCompleted() {
		t.Errorf("expected not completed after 2s with 3s ceiling")
	}

	session.ProcessFrame(vo.EvaluationResult{Passed: true, CalculatedAngle: 170}, 1*time.Second)
	if !session.IsCompleted() {
		t.Errorf("expected completed when reaching ceiling")
	}
	if session.CompletionReason() != "TIMEOUT" {
		t.Errorf("expected reason TIMEOUT, got '%s'", session.CompletionReason())
	}
	if session.Status() != ParticipantStatusCompleted {
		t.Errorf("expected status COMPLETED on timeout, got %v", session.Status())
	}
}

func TestSoloRefereeSession_Duration_HardCeilingAt30Minutes(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	session, err := NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, 60*time.Minute, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.RemainingCeilingSeconds() != 1800 {
		t.Errorf("expected ceiling to be hard-capped at 1800s (30m), got %d", session.RemainingCeilingSeconds())
	}

	// Simulate reaching 30 minutes (1800s)
	session.ProcessFrame(vo.EvaluationResult{Passed: true, FormScore: 100}, 1800*time.Second)

	if !session.IsCompleted() {
		t.Fatalf("expected session to complete at 30-minute ceiling")
	}
	if session.Status() != ParticipantStatusCompleted {
		t.Errorf("expected COMPLETED status, got %v", session.Status())
	}
	if session.ConfirmedValue() != 1800 {
		t.Errorf("expected confirmed value to be capped at 1800s, got %d", session.ConfirmedValue())
	}
	if session.RemainingCeilingSeconds() != 0 {
		t.Errorf("expected remaining ceiling to be 0s, got %d", session.RemainingCeilingSeconds())
	}

	// Try sending more frames after ceiling is reached -> must NOT accept any extra time!
	session.ProcessFrame(vo.EvaluationResult{Passed: true, FormScore: 100}, 10*time.Second)
	if session.ConfirmedValue() != 1800 {
		t.Errorf("expected confirmed value to remain 1800s, got %d", session.ConfirmedValue())
	}
	if session.TotalDurationSeconds() != 1800 {
		t.Errorf("expected total duration to remain capped at 1800s, got %d", session.TotalDurationSeconds())
	}
}

func TestSoloRefereeSession_Duration_DefaultCeilingIs30Minutes(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	session, err := NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, 0, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.RemainingCeilingSeconds() != 1800 {
		t.Errorf("expected default duration ceiling to be 1800s, got %d", session.RemainingCeilingSeconds())
	}
}

func TestSoloRefereeSession_WallClock_CeilingExpiresWithoutFrames(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	ceiling := 10 * time.Minute
	session, err := NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, ceiling, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ban đầu khi chưa gửi frame nào:
	if session.RemainingCeilingSeconds() != 1800 { // Duration challenge được hard-cap/default ở 1800s
		t.Errorf("expected ceiling 1800s, got %d", session.RemainingCeilingSeconds())
	}

	// Giả lập thời gian thực trôi qua 1801 giây mà người chơi KHÔNG gửi bất kỳ frame nào:
	session.SetStartedAt(time.Now().Add(-1801 * time.Second))

	// RemainingCeilingSeconds phải tự động giảm về 0:
	if rem := session.RemainingCeilingSeconds(); rem != 0 {
		t.Errorf("expected remaining ceiling to be 0s after wall-clock timeout, got %d", rem)
	}

	// TotalDurationSeconds phải phản ánh đúng thời gian trần đã trôi qua (1800s), không phải 0:
	if total := session.TotalDurationSeconds(); total != 1800 {
		t.Errorf("expected total duration to be 1800s, got %d", total)
	}
}

func TestSoloRefereeSession_WaitingState_DoesNotCountTime(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	session, err := NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, 0, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Người chơi đầu tiên vào phòng và chuyển sang trạng thái WAITING:
	session.SetStatus(ParticipantStatusWaiting)
	if !session.StartedAt().IsZero() {
		t.Errorf("expected startedAt to be zero while in WAITING status")
	}

	// Giả lập thời gian trôi qua trong lúc chờ đối thủ:
	time.Sleep(10 * time.Millisecond)
	if rem := session.RemainingCeilingSeconds(); rem != 1800 {
		t.Errorf("expected remaining ceiling to stay at 1800s while waiting, got %d", rem)
	}

	// Khi đối thủ vào phòng, chuyển sang ACTIVE -> startedAt được kích hoạt:
	session.Activate()
	if session.StartedAt().IsZero() {
		t.Errorf("expected startedAt to be non-zero after activation")
	}
}

func TestSoloRefereeSession_SpectatingState_WallElapsedContinues(t *testing.T) {
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "squat_01",
		ChallengeType: vo.ChallengeTypeReps,
		PhaseDetection: &vo.PhaseDetectionConfig{
			Metric: "knee_angle",
			Thresholds: vo.PhaseThresholds{
				Rest:       vo.ThresholdCondition{GT: 165.0},
				Transition: vo.ThresholdCondition{GT: 85.0, LTE: 165.0},
				Active:     vo.ThresholdCondition{LTE: 85.0},
			},
		},
	}
	ceiling := 300 * time.Second
	session, err := NewSoloRefereeSession("pushup_01", vo.ChallengeTypeReps, ceiling, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ban đầu là ACTIVE:
	if got, want := session.Status(), ParticipantStatusActive; got != want {
		t.Fatalf("status got %v, want %v", got, want)
	}

	// Giả lập người chơi hoàn thành bài tập sớm và chuyển sang trạng thái SPECTATING:
	session.MarkSpectating("USER_SUBMITTED")
	if got, want := session.Status(), ParticipantStatusSpectating; got != want {
		t.Fatalf("status got %v, want %v", got, want)
	}

	// Giả lập thời gian thực của trận đấu trôi qua 100 giây:
	session.SetStartedAt(time.Now().Add(-100 * time.Second))

	// RemainingCeilingSeconds phải tiếp tục giảm theo thời gian thực (300 - 100 = 200s), không bị đóng băng:
	if rem := session.RemainingCeilingSeconds(); rem > 200 || rem < 198 {
		t.Errorf("remaining ceiling got %d, want ~200s", rem)
	}

	// Giả lập trận đấu trôi qua quá thời gian trần (305 giây):
	session.SetStartedAt(time.Now().Add(-305 * time.Second))

	// RemainingCeilingSeconds phải chạm 0:
	if got, want := session.RemainingCeilingSeconds(), int32(0); got != want {
		t.Errorf("remaining ceiling after timeout got %d, want %d", got, want)
	}
}
