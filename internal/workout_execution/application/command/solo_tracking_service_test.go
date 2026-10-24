package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

type mockRuleProvider struct {
	rule  *vo.ExerciseMotionRule
	rules map[string]*vo.ExerciseMotionRule
	err   error
}

func (m *mockRuleProvider) GetRule(ctx context.Context, exerciseID string) (*vo.ExerciseMotionRule, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.rules != nil {
		return m.rules[exerciseID], nil
	}
	return m.rule, nil
}

type mockLiveSyncPort struct {
	myProgress   port.ParticipantProgress
	opponentProg *port.ParticipantProgress
	participants map[string]bool
	exerciseID   string
	regErr       error
	isReady      bool
}

func (m *mockLiveSyncPort) RegisterParticipant(ctx context.Context, matchID, userID, exerciseID string) error {
	if m.regErr != nil {
		return m.regErr
	}
	if m.exerciseID != "" && exerciseID != "" && m.exerciseID != exerciseID {
		return apperror.ErrMatchExerciseMismatch
	}
	if m.exerciseID == "" && exerciseID != "" {
		m.exerciseID = exerciseID
	}
	if m.participants == nil {
		m.participants = make(map[string]bool)
	}
	if !m.participants[userID] && len(m.participants) >= 2 {
		return apperror.ErrMatchRoomFull
	}
	m.participants[userID] = true
	return nil
}

func (m *mockLiveSyncPort) UnregisterParticipant(ctx context.Context, matchID, userID string) error {
	delete(m.participants, userID)
	if len(m.participants) == 0 {
		m.exerciseID = ""
	}
	return nil
}

func (m *mockLiveSyncPort) IsMatchReady(ctx context.Context, matchID string) (bool, error) {
	if m.isReady {
		return true, nil
	}
	return len(m.participants) >= 2, nil
}

func (m *mockLiveSyncPort) ParticipantCount(ctx context.Context, matchID string) (int, error) {
	return len(m.participants), nil
}

func (m *mockLiveSyncPort) PublishMyProgress(ctx context.Context, matchID string, progress port.ParticipantProgress) error {
	m.myProgress = progress
	return nil
}

func (m *mockLiveSyncPort) GetOpponentProgress(ctx context.Context, matchID, myUserID string) (*port.ParticipantProgress, error) {
	return m.opponentProg, nil
}

func (m *mockLiveSyncPort) SubscribeMatchUpdates(ctx context.Context, matchID, myUserID string) (<-chan port.ParticipantProgress, func(), error) {
	ch := make(chan port.ParticipantProgress, 1)
	return ch, func() {}, nil
}

type mockOutboxWriter struct {
	events []interface{}
}

func (m *mockOutboxWriter) WriteEvents(ctx context.Context, aggregateType, aggregateID string, events []interface{}) error {
	m.events = append(m.events, events...)
	return nil
}

func TestSoloTrackingService(t *testing.T) {
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
				ErrorCodeAbove: "ERR_BACK_ARCH",
			},
		},
	}

	ruleProv := &mockRuleProvider{rule: rule}
	liveSync := &mockLiveSyncPort{
		isReady: true,
		opponentProg: &port.ParticipantProgress{
			UserID:    "user-opponent",
			Value:     12,
			FormScore: 90.0,
			Status:    "ACTIVE",
		},
	}
	outbox := &mockOutboxWriter{}

	svc := NewSoloTrackingService(ruleProv, service.NewDefaultRuleEvaluator(), liveSync, outbox, nil, nil)

	referee, r, err := svc.InitializeSession(ctx, "match-1", "user-1", "plank_01", vo.ChallengeTypeDuration, 600, "plan-solo-test")
	if err != nil {
		t.Fatalf("failed to initialize session: %v", err)
	}
	if r == nil || referee == nil {
		t.Fatal("expected non-nil referee and rule")
	}

	// Frame 1: Valid
	landmarks := []vo.SkeletonPoint{
		{Index: 11, X: -1, Y: 0, Z: 0, Visibility: 0.9},
		{Index: 23, X: 0, Y: 0, Z: 0, Visibility: 0.9},
		{Index: 27, X: 1, Y: 0, Z: 0, Visibility: 0.9},
	}
	res, err := svc.ProcessFrame(ctx, referee, r, landmarks, 1*time.Second, "match-1", "user-1", "plank_01", "plan-solo-test", "side")
	if err != nil {
		t.Fatalf("unexpected error processing frame: %v", err)
	}

	if !res.IsValidForm {
		t.Errorf("expected valid form")
	}
	if res.MyConfirmedValue != 1 {
		t.Errorf("expected 1 second confirmed, got %d", res.MyConfirmedValue)
	}
	if res.OpponentValue != 12 {
		t.Errorf("expected opponent value 12, got %d", res.OpponentValue)
	}

	// FinalizeSession writes SoloTrackingAborted if not already completed (disconnect)
	err = svc.FinalizeSession(ctx, referee, "match-1", "user-1", "plank_01", "plan-solo-test")
	if err != nil {
		t.Fatalf("unexpected error finalizing session: %v", err)
	}

	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 outbox event written, got %d", len(outbox.events))
	}
	ev, ok := outbox.events[0].(*domainEvent.SoloTrackingAborted)
	if !ok {
		t.Fatalf("expected *domainEvent.SoloTrackingAborted, got %T", outbox.events[0])
	}
	if ev.PlanID != "plan-solo-test" {
		t.Errorf("expected PlanID plan-solo-test, got %s", ev.PlanID)
	}
	if ev.Reason != "USER_DISCONNECTED" {
		t.Errorf("expected reason USER_DISCONNECTED, got %s", ev.Reason)
	}

	// Test AbortSession directly (explicit surrender)
	outbox.events = nil
	referee2, _, _ := svc.InitializeSession(ctx, "match-2", "user-2", "plank_01", "DURATION", 600, "plan-abort-2")
	err = svc.AbortSession(ctx, referee2, "match-2", "user-2", "plank_01", "SURRENDER", "plan-abort-2")
	if err != nil {
		t.Fatalf("unexpected error aborting session: %v", err)
	}
	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 outbox event for abort, got %d", len(outbox.events))
	}
	abortEv, ok := outbox.events[0].(*domainEvent.SoloTrackingAborted)
	if !ok {
		t.Fatalf("expected *domainEvent.SoloTrackingAborted, got %T", outbox.events[0])
	}
	if abortEv.Reason != "SURRENDER" || abortEv.PlanID != "plan-abort-2" {
		t.Errorf("mismatch abort event: %+v", abortEv)
	}
}

func TestSoloTrackingService_InitializeSession_NilRule_ReturnsError(t *testing.T) {
	ctx := context.Background()
	ruleProv := &mockRuleProvider{rule: nil, err: nil}
	svc := NewSoloTrackingService(ruleProv, nil, nil, nil, nil, nil)

	referee, r, err := svc.InitializeSession(ctx, "match-3", "user-3", "unconfigured_exercise", "DURATION", 600, "plan-3")
	if err == nil {
		t.Fatalf("expected error when exercise rule is nil, got nil error")
	}
	if referee != nil || r != nil {
		t.Errorf("expected nil referee and rule on unconfigured exercise")
	}
}

func TestSoloTrackingService_EmptyPlanID_ReturnsError(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
	}
	ruleProv := &mockRuleProvider{rule: rule}
	svc := NewSoloTrackingService(ruleProv, nil, nil, nil, nil, nil)

	_, _, err := svc.InitializeSession(ctx, "match-empty-plan", "user-1", "plank_01", vo.ChallengeTypeDuration, 600, "")
	if err == nil {
		t.Fatal("expected error when plan_id is empty, got nil")
	}
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Errorf("got %v, want %v", err, apperror.ErrInvalidInput)
	}
}

func TestSoloTrackingService_RoomFull_BlocksThirdParticipant(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
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

func TestSoloTrackingService_ExerciseMismatch(t *testing.T) {
	ctx := context.Background()
	rulePlank := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		MinConfidence: 0.5,
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

func TestSoloTrackingService_WaitingOpponent_DoesNotCountReps(t *testing.T) {
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

type mockSoloSessionRepo struct {
	sessions map[string]*aggregate.WorkoutSession
}

func newMockSoloSessionRepo() *mockSoloSessionRepo {
	return &mockSoloSessionRepo{sessions: make(map[string]*aggregate.WorkoutSession)}
}

func (m *mockSoloSessionRepo) Save(ctx context.Context, session *aggregate.WorkoutSession) error {
	m.sessions[session.ID()] = session
	return nil
}

func (m *mockSoloSessionRepo) FindByID(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	return m.sessions[id], nil
}

func (m *mockSoloSessionRepo) FindByIDForUpdate(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	return m.sessions[id], nil
}

func (m *mockSoloSessionRepo) FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error) {
	return nil, nil
}

func (m *mockSoloSessionRepo) FindTimedOutSessions(ctx context.Context, maxDurationMinutes int) ([]*aggregate.WorkoutSession, error) {
	return nil, nil
}

func (m *mockSoloSessionRepo) FindHistoryByUserID(ctx context.Context, userID string, limit, offset int) ([]*aggregate.WorkoutSession, error) {
	return nil, nil
}

func (m *mockSoloSessionRepo) FindSessionsWithCriticalInactivity(ctx context.Context, inactivityThreshold time.Duration) ([]*aggregate.WorkoutSession, error) {
	return nil, nil
}

func TestSoloTrackingService_WorkoutSessionPersistence(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "squat_01",
		ChallengeType: vo.ChallengeTypeReps,
		MinConfidence: 0.5,
		PhaseDetection: &vo.PhaseDetectionConfig{
			Metric: "knee_angle",
			Thresholds: vo.PhaseThresholds{
				Rest:       vo.ThresholdCondition{GT: 165.0},
				Transition: vo.ThresholdCondition{GT: 85.0, LTE: 165.0},
				Active:     vo.ThresholdCondition{LTE: 85.0},
			},
		},
		Rules: []vo.RuleCriterion{
			{
				Name:    "knee",
				Formula: vo.FormulaAngle3D,
				Joints:  []int{23, 25, 27},
				ValidRange: vo.ValidAngleRange{
					Min: 70.0,
					Max: 180.0,
				},
			},
		},
	}

	ruleProv := &mockRuleProvider{rule: rule}
	liveSync := &mockLiveSyncPort{isReady: true}
	outbox := &mockOutboxWriter{}
	sessionRepo := newMockSoloSessionRepo()

	svc := NewSoloTrackingService(ruleProv, service.NewDefaultRuleEvaluator(), liveSync, outbox, sessionRepo, nil)

	// 1. Initialize session -> must create WorkoutSession in DB with StatusInProgress
	referee, _, err := svc.InitializeSession(ctx, "match-persist-1", "user-persist", "squat_01", vo.ChallengeTypeReps, 300, "plan-custom-1")
	if err != nil {
		t.Fatalf("initialize session error: %v", err)
	}

	sessionID := referee.WorkoutSessionID()
	if sessionID == "" {
		t.Fatalf("expected referee to have a non-empty WorkoutSessionID")
	}

	dbSession := sessionRepo.sessions[sessionID]
	if dbSession == nil {
		t.Fatalf("expected session to be saved in DB repository")
	}
	if dbSession.Status() != aggregate.StatusInProgress {
		t.Errorf("got status %s, want IN_PROGRESS", dbSession.Status())
	}
	if dbSession.UserID() != "user-persist" {
		t.Errorf("got user %s, want user-persist", dbSession.UserID())
	}
	if dbSession.PlanID() != "plan-custom-1" {
		t.Errorf("got plan %s, want plan-custom-1", dbSession.PlanID())
	}

	// 2. Mark completed and finalize -> must transition to StatusCompleted and log set
	referee.MarkCompleted("GOAL_REACHED")
	err = svc.FinalizeSession(ctx, referee, "match-persist-1", "user-persist", "squat_01", "plan-custom-1")
	if err != nil {
		t.Fatalf("finalize session error: %v", err)
	}

	updatedDBSess := sessionRepo.sessions[sessionID]
	if updatedDBSess.Status() != aggregate.StatusCompleted {
		t.Errorf("got status %s, want COMPLETED", updatedDBSess.Status())
	}
	if len(updatedDBSess.Sets()) != 1 {
		t.Errorf("got %d sets logged in session, want 1", len(updatedDBSess.Sets()))
	}

	// 3. Test Abort on another session
	referee2, _, err := svc.InitializeSession(ctx, "match-persist-2", "user-abort", "squat_01", vo.ChallengeTypeReps, 300, "plan-abort-persist")
	if err != nil {
		t.Fatalf("init session 2 error: %v", err)
	}
	sessID2 := referee2.WorkoutSessionID()
	err = svc.AbortSession(ctx, referee2, "match-persist-2", "user-abort", "squat_01", "GAVE_UP", "match-persist-2")
	if err != nil {
		t.Fatalf("abort session error: %v", err)
	}
	abortedSess := sessionRepo.sessions[sessID2]
	if abortedSess.Status() != aggregate.StatusAborted {
		t.Errorf("got status %s, want ABORTED", abortedSess.Status())
	}
}

func TestSoloTrackingService_SubmitSession(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
	}
	ruleProv := &mockRuleProvider{rule: rule}
	outbox := &mockOutboxWriter{}
	liveSync := &mockLiveSyncPort{
		isReady: true,
	}

	svc := NewSoloTrackingService(ruleProv, service.NewDefaultRuleEvaluator(), liveSync, outbox, nil, nil)

	referee, _, err := svc.InitializeSession(ctx, "match-submit-1", "user-submit", "plank_01", vo.ChallengeTypeDuration, 300, "plan-submit")
	if err != nil {
		t.Fatalf("init session error: %v", err)
	}

	// 1. Submit when opponent is still waiting -> transitions to SPECTATING
	res, err := svc.SubmitSession(ctx, referee, "match-submit-1", "user-submit", "plank_01", "plan-submit")
	if err != nil {
		t.Fatalf("submit session error: %v", err)
	}
	if res.MyStatus != "SPECTATING" {
		t.Errorf("expected status SPECTATING, got %s", res.MyStatus)
	}
	if res.MatchEnded {
		t.Errorf("expected MatchEnded=false when opponent is waiting, got true")
	}

	// 2. Opponent completes match -> submit again -> MatchEnded = true
	liveSync.opponentProg = &port.ParticipantProgress{
		UserID: "opponent-1",
		Status: "COMPLETED",
	}
	res2, err := svc.SubmitSession(ctx, referee, "match-submit-1", "user-submit", "plank_01", "plan-submit")
	if err != nil {
		t.Fatalf("submit session 2 error: %v", err)
	}
	if res2.MyStatus != "COMPLETED" {
		t.Errorf("expected status COMPLETED, got %s", res2.MyStatus)
	}
	if !res2.MatchEnded {
		t.Errorf("expected MatchEnded=true when opponent is completed, got false")
	}
}
