package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

func TestCreateAdhocSessionHandler_Success(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	today := now.Truncate(24 * time.Hour)

	// Setup domain: create a simple roadmap with one day
	roadmapID := "rm-123"
	userID := "user-123"

	info := &roadmap.Info{
		RoadmapID: roadmapID,
		UserID:    userID,
		Status:    roadmap.StatusActive,
		StartDate: today,
		EndDate:   today.AddDate(0, 0, 27),
	}

	dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
		DayPlanID:     "d-123",
		WeekPlanID:    "w-123",
		RoadmapID:     roadmapID,
		UserID:        userID,
		ScheduledDate: today,
	})
	if err != nil {
		t.Fatalf("NewDayPlan: %v", err)
	}
	wp, err := roadmap.RehydrateWeekPlan(&roadmap.WeekPlanInfo{
		WeekPlanID: "w-123",
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

	// Mock clock
	mockClock := &clockMock{now: now}

	// Mock IDGenerator
	idCounter := 0
	mockIDGen := &idGeneratorMock{
		fn: func() string {
			idCounter++
			return "adhoc-session-" + string(rune(idCounter))
		},
	}

	// Mock ExerciseCatalogReader
	mockExercises := &exerciseCatalogMock{
		fn: func(ctx context.Context, exerciseID string) (port.Exercise, error) {
			return port.Exercise{
				ExerciseID:  exerciseID,
				Name:        "Bench Press",
				MuscleGroup: "Chest",
				Equipment:   "Barbell",
				Difficulty:  "Intermediate",
			}, nil
		},
	}

	// Mock RoadmapRepository
	mockRepo := &roadmapRepositoryMock{
		findActiveByUserFn: func(ctx context.Context, uid string) (*roadmap.Roadmap, error) {
			return rm, nil
		},
		saveFn: func(ctx context.Context, r *roadmap.Roadmap) error {
			return nil
		},
	}

	// Mock TransactionManager
	mockTx := &transactionManagerMock{
		fn: func(ctx context.Context, f func(context.Context) error) error {
			return f(ctx)
		},
	}

	handler := NewCreateAdhocSessionHandler(
		mockTx,
		mockRepo,
		mockExercises,
		mockIDGen,
		mockClock,
	)

	result, err := handler.Handle(ctx, CreateAdhocSessionCommand{
		UserID:      userID,
		ExerciseIDs: []string{"ex-1", "ex-2"},
	})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.SessionPlan == nil {
		t.Fatal("expected session plan in result")
	}

	spInfo := result.SessionPlan.Info()
	if spInfo.Source != roadmap.SessionPlanSourceAdHoc {
		t.Errorf("expected source USER_ADHOC, got %s", spInfo.Source)
	}

	if spInfo.UserID != userID {
		t.Errorf("expected user_id %s, got %s", userID, spInfo.UserID)
	}

	if spInfo.Status != roadmap.SessionPlanStatusPending {
		t.Errorf("expected status PENDING, got %s", spInfo.Status)
	}
}

func TestCreateAdhocSessionHandler_NoActiveRoadmap(t *testing.T) {
	ctx := context.Background()
	mockClock := &clockMock{now: time.Now()}
	mockTx := &transactionManagerMock{
		fn: func(ctx context.Context, f func(context.Context) error) error {
			return f(ctx)
		},
	}

	mockRepo := &roadmapRepositoryMock{
		findActiveByUserFn: func(ctx context.Context, uid string) (*roadmap.Roadmap, error) {
			return nil, roadmap.ErrRoadmapNotFound
		},
	}

	handler := NewCreateAdhocSessionHandler(
		mockTx,
		mockRepo,
		nil,
		nil,
		mockClock,
	)

	_, err := handler.Handle(ctx, CreateAdhocSessionCommand{
		UserID:      "user-123",
		ExerciseIDs: []string{"ex-1"},
	})

	if err == nil {
		t.Fatal("expected error for no active roadmap")
	}
}

func TestCreateAdhocSessionHandler_NoExerciseIDs(t *testing.T) {
	ctx := context.Background()
	mockClock := &clockMock{now: time.Now()}
	mockTx := &transactionManagerMock{
		fn: func(ctx context.Context, f func(context.Context) error) error {
			return f(ctx)
		},
	}

	handler := NewCreateAdhocSessionHandler(
		mockTx,
		nil,
		nil,
		nil,
		mockClock,
	)

	_, err := handler.Handle(ctx, CreateAdhocSessionCommand{
		UserID:      "user-123",
		ExerciseIDs: []string{},
	})

	if err == nil {
		t.Fatal("expected error for no exercise_ids")
	}
}

// Test mocks

type clockMock struct {
	now time.Time
}

func (m *clockMock) Now() time.Time { return m.now }

type idGeneratorMock struct {
	fn func() string
}

func (m *idGeneratorMock) NewID() string { return m.fn() }

type exerciseCatalogMock struct {
	fn func(context.Context, string) (port.Exercise, error)
}

func (m *exerciseCatalogMock) GetByID(ctx context.Context, id string) (port.Exercise, error) {
	return m.fn(ctx, id)
}

func (m *exerciseCatalogMock) SearchByFilter(ctx context.Context, f *port.ExerciseFilter) ([]port.Exercise, error) {
	return nil, nil
}

type roadmapRepositoryMock struct {
	findActiveByUserFn func(context.Context, string) (*roadmap.Roadmap, error)
	saveFn             func(context.Context, *roadmap.Roadmap) error
}

func (m *roadmapRepositoryMock) FindActiveByUser(ctx context.Context, uid string) (*roadmap.Roadmap, error) {
	return m.findActiveByUserFn(ctx, uid)
}

func (m *roadmapRepositoryMock) Save(ctx context.Context, r *roadmap.Roadmap) error {
	return m.saveFn(ctx, r)
}

func (m *roadmapRepositoryMock) FindByID(ctx context.Context, rid string) (*roadmap.Roadmap, error) {
	return nil, nil
}

func (m *roadmapRepositoryMock) ListByUser(ctx context.Context, uid string, status roadmap.Status, limit, offset int) ([]*roadmap.Roadmap, error) {
	return nil, nil
}

func (m *roadmapRepositoryMock) FindSessionByID(ctx context.Context, sid string) (*roadmap.Roadmap, error) {
	return nil, nil
}

func (m *roadmapRepositoryMock) FindPendingSessionsByDate(ctx context.Context, targetDate time.Time) ([]*roadmap.SessionPlanInfo, error) {
	return nil, nil
}

type transactionManagerMock struct {
	fn func(context.Context, func(context.Context) error) error
}

func (m *transactionManagerMock) WithTransaction(ctx context.Context, f func(context.Context) error) error {
	return m.fn(ctx, f)
}

func findOrCreateDayPlan(t *testing.T, week *roadmap.WeekPlan, date time.Time) *roadmap.DayPlan {
	for _, day := range week.Days() {
		if day.Info().ScheduledDate.Truncate(24*time.Hour) == date.Truncate(24*time.Hour) {
			return day
		}
	}

	// If not found, this is a test setup issue - weekplan should have days
	t.Fatal("no matching day found in week")
	return nil
}
