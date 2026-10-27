package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

type mockSoloSessionRepo struct {
	sessions      map[string]*aggregate.WorkoutSession
	activeSession *aggregate.WorkoutSession
}

func newMockSoloSessionRepo() *mockSoloSessionRepo {
	return &mockSoloSessionRepo{sessions: make(map[string]*aggregate.WorkoutSession)}
}

func (m *mockSoloSessionRepo) Save(ctx context.Context, session *aggregate.WorkoutSession) error {
	m.sessions[session.ID()] = session
	if session.Status() == aggregate.StatusInProgress {
		m.activeSession = session
	} else if m.activeSession != nil && m.activeSession.ID() == session.ID() {
		m.activeSession = nil
	}
	return nil
}

func (m *mockSoloSessionRepo) FindByID(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	return m.sessions[id], nil
}

func (m *mockSoloSessionRepo) FindByIDForUpdate(ctx context.Context, id string) (*aggregate.WorkoutSession, error) {
	return m.sessions[id], nil
}

func (m *mockSoloSessionRepo) FindActiveSessionByUserID(ctx context.Context, userID string) (*aggregate.WorkoutSession, error) {
	return m.activeSession, nil
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

func sampleRepRule(exerciseID string) *vo.ExerciseMotionRule {
	return &vo.ExerciseMotionRule{
		ExerciseID:    exerciseID,
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
}

func TestSoloTrackingPersistence_WorkoutSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	rule := sampleRepRule("squat_01")
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
	err = svc.AbortSession(ctx, referee2, "match-persist-2", "user-abort", "squat_01", "GAVE_UP", "plan-abort-persist")
	if err != nil {
		t.Fatalf("abort session error: %v", err)
	}
	abortedSess := sessionRepo.sessions[sessID2]
	if abortedSess.Status() != aggregate.StatusAborted {
		t.Errorf("got status %s, want ABORTED", abortedSess.Status())
	}
}

func TestSoloTrackingPersistence_ReuseExistingActiveSessionSamePlan(t *testing.T) {
	ctx := context.Background()
	rule := sampleRepRule("squat_01")
	ruleProv := &mockRuleProvider{rule: rule}
	sessionRepo := newMockSoloSessionRepo()
	outbox := &mockOutboxWriter{}

	existingSess, err := aggregate.NewWorkoutSession("existing-session-123", "user-reuse", "plan-same")
	if err != nil {
		t.Fatalf("create existing session error: %v", err)
	}
	_ = sessionRepo.Save(ctx, existingSess)

	svc := NewSoloTrackingService(ruleProv, nil, nil, outbox, sessionRepo, nil)

	referee, _, err := svc.InitializeSession(ctx, "match-reuse", "user-reuse", "squat_01", vo.ChallengeTypeReps, 300, "plan-same")
	if err != nil {
		t.Fatalf("initialize session error: %v", err)
	}

	if referee.WorkoutSessionID() != "existing-session-123" {
		t.Errorf("expected reused session ID existing-session-123, got %s", referee.WorkoutSessionID())
	}
	if existingSess.Status() != aggregate.StatusInProgress {
		t.Errorf("expected session to remain IN_PROGRESS, got %s", existingSess.Status())
	}
}

func TestSoloTrackingPersistence_AbortExistingActiveSessionDifferentPlan(t *testing.T) {
	ctx := context.Background()
	rule := sampleRepRule("squat_01")
	ruleProv := &mockRuleProvider{rule: rule}
	sessionRepo := newMockSoloSessionRepo()
	outbox := &mockOutboxWriter{}

	oldSess, err := aggregate.NewWorkoutSession("old-session-456", "user-diff", "plan-old")
	if err != nil {
		t.Fatalf("create old session error: %v", err)
	}
	_ = sessionRepo.Save(ctx, oldSess)

	svc := NewSoloTrackingService(ruleProv, nil, nil, outbox, sessionRepo, nil)

	referee, _, err := svc.InitializeSession(ctx, "match-diff", "user-diff", "squat_01", vo.ChallengeTypeReps, 300, "plan-new")
	if err != nil {
		t.Fatalf("initialize session error: %v", err)
	}

	if referee.WorkoutSessionID() == "old-session-456" {
		t.Errorf("expected new session ID, got old one")
	}
	if oldSess.Status() != aggregate.StatusAborted {
		t.Errorf("expected old session to be ABORTED, got %s", oldSess.Status())
	}
}

func TestSoloTrackingPersistence_NilRepo_WritesOutboxDirectly(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
	}
	ruleProv := &mockRuleProvider{rule: rule}
	outbox := &mockOutboxWriter{}

	svc := NewSoloTrackingService(ruleProv, nil, nil, outbox, nil, nil)

	referee, err := service.NewSoloRefereeSession("plank_01", vo.ChallengeTypeDuration, 300*time.Second, rule)
	if err != nil {
		t.Fatalf("create referee session error: %v", err)
	}
	referee.MarkCompleted("GOAL_REACHED")

	err = svc.completeWorkoutSessionInDB(ctx, referee, "match-nil-repo", "user-nil", "plank_01", "plan-nil")
	if err != nil {
		t.Fatalf("completeWorkoutSessionInDB error: %v", err)
	}

	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outbox.events))
	}
	soloEv, ok := outbox.events[0].(*domainEvent.SoloTrackingCompleted)
	if !ok {
		t.Fatalf("expected *domainEvent.SoloTrackingCompleted, got %T", outbox.events[0])
	}
	if soloEv.MatchID != "match-nil-repo" {
		t.Errorf("expected MatchID match-nil-repo, got %s", soloEv.MatchID)
	}
}
