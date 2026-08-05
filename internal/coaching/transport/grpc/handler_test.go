package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/application/query"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	pbmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
)

func TestServer_CreateAdhocSessionPlan(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	today := now.Truncate(24 * time.Hour)

	// Setup domain
	roadmapID := "rm-100"
	userID := "user-100"

	info := &roadmap.Info{
		RoadmapID: roadmapID,
		UserID:    userID,
		Status:    roadmap.StatusActive,
		StartDate: today,
		EndDate:   today.AddDate(0, 0, 27),
	}

	dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
		DayPlanID:     "d-100",
		WeekPlanID:    "w-100",
		RoadmapID:     roadmapID,
		UserID:        userID,
		ScheduledDate: today,
	})
	if err != nil {
		t.Fatalf("NewDayPlan: %v", err)
	}

	wp, err := roadmap.RehydrateWeekPlan(&roadmap.WeekPlanInfo{
		WeekPlanID: "w-100",
		RoadmapID:  roadmapID,
		UserID:     userID,
		WeekNumber: 1,
		Phase:      roadmap.PhaseAccumulation,
		StartDate:  today,
		EndDate:    today.AddDate(0, 0, 6),
	}, []*roadmap.DayPlan{dp})
	if err != nil {
		t.Fatalf("NewWeekPlan: %v", err)
	}

	rm, err := roadmap.NewRoadmap(info, []*roadmap.WeekPlan{wp}, now)
	if err != nil {
		t.Fatalf("NewRoadmap: %v", err)
	}

	mockClock := &testClock{now: now}
	mockIDGen := &testIDGen{id: "session-adhoc-777"}
	mockCatalog := &testCatalog{exName: "Bench Press"}
	mockRepo := &testRepo{rm: rm}
	mockTx := &testTx{}

	createAdhocHandler := command.NewCreateAdhocSessionHandler(
		mockTx,
		mockRepo,
		mockCatalog,
		mockIDGen,
		mockClock,
	)

	server := NewServer(nil, nil, createAdhocHandler, nil)

	req := &pbmsg.CreateAdhocSessionPlanRequest{
		UserId:      userID,
		ExerciseIds: []string{"ex-1"},
	}

	resp, err := server.CreateAdhocSessionPlan(ctx, req)
	if err != nil {
		t.Fatalf("CreateAdhocSessionPlan failed: %v", err)
	}

	if resp == nil || resp.GetSessionPlan() == nil {
		t.Fatal("expected non-nil SessionPlan response")
	}

	sp := resp.GetSessionPlan()
	if sp.GetSessionPlanId() == "" {
		t.Error("expected non-empty SessionPlanId in response")
	}

	if sp.GetUserId() != userID {
		t.Errorf("expected UserId %s, got %s", userID, sp.GetUserId())
	}

	if sp.GetStatus() != pbmsg.SessionPlanStatus_SESSION_PLAN_STATUS_PENDING {
		t.Errorf("expected PENDING status, got %v", sp.GetStatus())
	}

	if sp.GetSource() != pbmsg.SessionPlanSource_SESSION_PLAN_SOURCE_USER_ADHOC {
		t.Errorf("expected USER_ADHOC source, got %v", sp.GetSource())
	}
}

func TestServer_CreateAdhocSessionPlan_MissingUserId(t *testing.T) {
	server := NewServer(nil, nil, nil, nil)
	_, err := server.CreateAdhocSessionPlan(context.Background(), &pbmsg.CreateAdhocSessionPlanRequest{})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestServer_SuggestAdHocSession(t *testing.T) {
	ctx := context.Background()
	userID := "user-200"

	mockAgent := &testCoachAgent{
		suggestResult: port.SuggestedSession{
			MuscleGroups: []string{"Chest", "Triceps"},
			Reasoning:    "JIT Ad-Hoc session for chest focus",
			EstimatedRPE: 7.5,
		},
	}

	suggestHandler := query.NewSuggestAdHocSessionHandler(nil, mockAgent, &testClock{now: time.Now()})
	server := NewServer(nil, nil, nil, nil).WithSuggestAdHoc(suggestHandler)

	req := &pbmsg.SuggestAdHocSessionRequest{
		UserId: userID,
		Hint: &pbmsg.AdHocHint{
			MuscleGroups:    []string{"Chest"},
			DurationMinutes: 45,
			IntensityHint:   "normal",
		},
	}

	resp, err := server.SuggestAdHocSession(ctx, req)
	if err != nil {
		t.Fatalf("SuggestAdHocSession failed: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if resp.GetReasoning() != "JIT Ad-Hoc session for chest focus" {
		t.Errorf("unexpected reasoning: %s", resp.GetReasoning())
	}

	if resp.GetEstimatedRpe() != 7.5 {
		t.Errorf("expected RPE 7.5, got %f", resp.GetEstimatedRpe())
	}
}

func TestServer_SuggestAdHocSession_MissingUserId(t *testing.T) {
	server := NewServer(nil, nil, nil, nil)
	_, err := server.SuggestAdHocSession(context.Background(), &pbmsg.SuggestAdHocSessionRequest{})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

// Mocks

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

type testIDGen struct{ id string }

func (g *testIDGen) NewID() string { return g.id }

type testCatalog struct{ exName string }

func (c *testCatalog) GetByID(ctx context.Context, id string) (port.Exercise, error) {
	return port.Exercise{
		ExerciseID:  id,
		Name:        c.exName,
		MuscleGroup: "Chest",
		Equipment:   "Barbell",
	}, nil
}

func (c *testCatalog) SearchByFilter(ctx context.Context, f *port.ExerciseFilter) ([]port.Exercise, error) {
	return nil, nil
}

type testRepo struct{ rm *roadmap.Roadmap }

func (r *testRepo) FindActiveByUser(ctx context.Context, uid string) (*roadmap.Roadmap, error) {
	return r.rm, nil
}
func (r *testRepo) Save(ctx context.Context, rm *roadmap.Roadmap) error { return nil }
func (r *testRepo) FindByID(ctx context.Context, id string) (*roadmap.Roadmap, error) {
	return nil, nil
}
func (r *testRepo) ListByUser(ctx context.Context, uid string, status roadmap.Status, limit, offset int) ([]*roadmap.Roadmap, error) {
	return nil, nil
}
func (r *testRepo) FindSessionByID(ctx context.Context, sid string) (*roadmap.Roadmap, error) {
	return nil, nil
}

type testTx struct{}

func (t *testTx) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type testCoachAgent struct {
	suggestResult port.SuggestedSession
}

func (a *testCoachAgent) GenerateRoadmap(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	return nil, nil
}
func (a *testCoachAgent) RegeneratePending(ctx context.Context, userID string, sessionIDs []string) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}
func (a *testCoachAgent) Adapt(ctx context.Context, userID string, decisionReason string) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}
func (a *testCoachAgent) SuggestAdHocSession(ctx context.Context, userID string, hint *port.AdHocHint) (port.SuggestedSession, error) {
	return a.suggestResult, nil
}
