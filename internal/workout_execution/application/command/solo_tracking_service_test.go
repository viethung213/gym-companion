package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
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

func (m *mockLiveSyncPort) DelRoom(ctx context.Context, matchID string) error {
	m.participants = nil
	return nil
}

func (m *mockLiveSyncPort) ParticipantCount(ctx context.Context, matchID string) (int, error) {
	return len(m.participants), nil
}

func (m *mockLiveSyncPort) PublishMyProgress(ctx context.Context, matchID string, progress port.ParticipantProgress) error {
	m.myProgress = progress
	return nil
}

func (m *mockLiveSyncPort) GetMyProgress(ctx context.Context, matchID, userID string) (*port.ParticipantProgress, error) {
	if m.myProgress.UserID == userID {
		return &m.myProgress, nil
	}
	return nil, nil
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

	if len(outbox.events) != 2 {
		t.Fatalf("expected 2 outbox events written (RoomReady + TrackingAborted), got %d", len(outbox.events))
	}
	readyEv, ok := outbox.events[0].(*domainEvent.SoloMatchRoomReady)
	if !ok {
		t.Fatalf("expected *domainEvent.SoloMatchRoomReady, got %T", outbox.events[0])
	}
	if readyEv.MatchID != "match-1" {
		t.Errorf("expected MatchID match-1, got %s", readyEv.MatchID)
	}

	ev, ok := outbox.events[1].(*domainEvent.SoloTrackingAborted)
	if !ok {
		t.Fatalf("expected *domainEvent.SoloTrackingAborted, got %T", outbox.events[1])
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
	if len(outbox.events) != 2 {
		t.Fatalf("expected 2 outbox events for abort (RoomReady + Aborted), got %d", len(outbox.events))
	}
	abortEv, ok := outbox.events[1].(*domainEvent.SoloTrackingAborted)
	if !ok {
		t.Fatalf("expected *domainEvent.SoloTrackingAborted, got %T", outbox.events[1])
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

func TestSoloTrackingService_SubmitSession(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		Rules:         []vo.RuleCriterion{{Name: "body_incline"}},
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

func TestSoloTrackingService_ForceCompleteSessionOnCeiling(t *testing.T) {
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

	err = svc.ForceCompleteSessionOnCeiling(ctx, referee, "match-ceiling", "user-ceil", "plank_01", "plan-ceil")
	if err != nil {
		t.Fatalf("force complete error: %v", err)
	}

	if !referee.IsCompleted() {
		t.Errorf("expected referee to be marked completed")
	}
	if referee.CompletionReason() != "TIMEOUT" {
		t.Errorf("expected completion reason TIMEOUT, got %s", referee.CompletionReason())
	}
	if len(outbox.events) != 1 {
		t.Fatalf("expected 1 outbox event for SoloTrackingCompleted, got %d", len(outbox.events))
	}
}

func TestSoloTrackingService_ForceCompleteSessionOnCeiling_WhenSpectating(t *testing.T) {
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

	// Người chơi submit sớm và đang ở trạng thái SPECTATING:
	referee.MarkSpectating("USER_SUBMITTED")
	if got, want := referee.Status(), service.ParticipantStatusSpectating; got != want {
		t.Fatalf("status got %v, want %v", got, want)
	}

	err = svc.ForceCompleteSessionOnCeiling(ctx, referee, "match-ceiling-spec", "user-ceil-spec", "plank_01", "plan-ceil")
	if err != nil {
		t.Fatalf("force complete error: %v", err)
	}

	// Trạng thái phải chuyển thành COMPLETED và giữ nguyên completionReason:
	if got, want := referee.Status(), service.ParticipantStatusCompleted; got != want {
		t.Errorf("referee status got %v, want %v", got, want)
	}
	if got, want := referee.CompletionReason(), "USER_SUBMITTED"; got != want {
		t.Errorf("referee completion reason got %s, want %s", got, want)
	}
}

func TestSoloTrackingService_SubscribeAndOpponentProgress(t *testing.T) {
	ctx := context.Background()
	liveSync := &mockLiveSyncPort{
		opponentProg: &port.ParticipantProgress{
			UserID: "opp-123",
			Value:  15,
		},
	}
	svc := NewSoloTrackingService(nil, nil, liveSync, nil, nil, nil)

	ch, unsub, err := svc.SubscribeMatchUpdates(ctx, "match-sub", "user-sub")
	if err != nil {
		t.Fatalf("subscribe error: %v", err)
	}
	if ch == nil || unsub == nil {
		t.Errorf("expected non-nil channel and unsubscribe function")
	}
	unsub()

	opp, err := svc.GetOpponentProgress(ctx, "match-sub", "user-sub")
	if err != nil {
		t.Fatalf("get opponent progress error: %v", err)
	}
	if opp == nil || opp.UserID != "opp-123" || opp.Value != 15 {
		t.Errorf("unexpected opponent progress: %+v", opp)
	}
}

func TestSoloTrackingService_HandleParticipantDisconnect_GracePeriodExpired(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		MinConfidence: 0.5,
		Rules: []vo.RuleCriterion{
			{
				Name:       "spine",
				ValidRange: vo.ValidAngleRange{Min: 165, Max: 195},
			},
		},
	}
	ruleProv := &mockRuleProvider{rule: rule}
	liveSync := &mockLiveSyncPort{}
	outbox := &mockOutboxWriter{}

	svc := NewSoloTrackingService(ruleProv, nil, liveSync, outbox, nil, nil)
	svc.SetDisconnectGracePeriodForTesting(50 * time.Millisecond)

	referee, _, err := svc.InitializeSession(ctx, "match-disc-1", "user-disc-1", "plank_01", "DURATION", 600, "plan-disc-1")
	if err != nil {
		t.Fatalf("failed to initialize session: %v", err)
	}

	outbox.events = nil

	// Disconnect triggers grace period
	err = svc.HandleParticipantDisconnect(ctx, referee, "match-disc-1", "user-disc-1", "plank_01", "plan-disc-1")
	if err != nil {
		t.Fatalf("HandleParticipantDisconnect error: %v", err)
	}

	// Wait for grace period to expire
	time.Sleep(120 * time.Millisecond)

	if !referee.IsFinalized() {
		t.Errorf("expected referee to be finalized after disconnect grace period expired")
	}

	if len(outbox.events) == 0 {
		t.Fatalf("expected SoloTrackingAborted event in outbox, got 0")
	}
	abortEv, ok := outbox.events[len(outbox.events)-1].(*domainEvent.SoloTrackingAborted)
	if !ok {
		t.Fatalf("expected *domainEvent.SoloTrackingAborted, got %T", outbox.events[len(outbox.events)-1])
	}
	if abortEv.Reason != "USER_DISCONNECTED" {
		t.Errorf("expected reason USER_DISCONNECTED, got %s", abortEv.Reason)
	}
	if abortEv.UserID != "user-disc-1" {
		t.Errorf("expected userID user-disc-1, got %s", abortEv.UserID)
	}
}

func TestSoloTrackingService_HandleParticipantDisconnect_Reconnected(t *testing.T) {
	ctx := context.Background()
	rule := &vo.ExerciseMotionRule{
		ExerciseID:    "plank_01",
		ChallengeType: vo.ChallengeTypeDuration,
		MinConfidence: 0.5,
		Rules: []vo.RuleCriterion{
			{
				Name:       "spine",
				ValidRange: vo.ValidAngleRange{Min: 165, Max: 195},
			},
		},
	}
	ruleProv := &mockRuleProvider{rule: rule}
	liveSync := &mockLiveSyncPort{}
	outbox := &mockOutboxWriter{}

	svc := NewSoloTrackingService(ruleProv, nil, liveSync, outbox, nil, nil)
	svc.SetDisconnectGracePeriodForTesting(100 * time.Millisecond)

	referee, _, err := svc.InitializeSession(ctx, "match-rec-1", "user-rec-1", "plank_01", "DURATION", 600, "plan-rec-1")
	if err != nil {
		t.Fatalf("failed to initialize session: %v", err)
	}

	// Disconnect triggers grace period
	err = svc.HandleParticipantDisconnect(ctx, referee, "match-rec-1", "user-rec-1", "plank_01", "plan-rec-1")
	if err != nil {
		t.Fatalf("HandleParticipantDisconnect error: %v", err)
	}

	// Reconnect quickly at 20ms
	time.Sleep(20 * time.Millisecond)
	_, _, err = svc.InitializeSession(ctx, "match-rec-1", "user-rec-1", "plank_01", "DURATION", 600, "plan-rec-1")
	if err != nil {
		t.Fatalf("failed to reconnect InitializeSession: %v", err)
	}

	// Wait past the original 100ms grace period
	time.Sleep(150 * time.Millisecond)

	// Since user reconnected, referee should NOT be finalized as aborted
	if referee.IsFinalized() {
		t.Errorf("expected referee NOT to be finalized when user reconnected within grace period")
	}
}
