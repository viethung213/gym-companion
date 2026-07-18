package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestGenerateDailyPlan(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	repository := newDailyPlanRepository(t, startDate, nil)
	searcher := &exerciseSearcherStub{candidates: []port.ExerciseCandidate{
		{ID: "push-up"},
		{ID: "triceps-extension"},
	}}
	handler := NewGenerateDailyPlanHandler(
		repository,
		searcher,
		fixedClock{value: startDate.Add(time.Hour)},
		&idGeneratorStub{ids: []string{"daily-plan-1"}},
	)

	plan, err := handler.Handle(context.Background(), &GenerateDailyPlan{
		UserID:           "user-1",
		WeeklyScheduleID: "schedule-1",
		ScheduledDate:    startDate,
	})
	if err != nil {
		t.Fatalf("generate daily plan: %v", err)
	}
	if plan.ID != "daily-plan-1" || len(plan.Exercises) != 2 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if plan.Exercises[0].Sets != 2 || plan.Exercises[0].Reps != 12 {
		t.Fatalf("unexpected beginner prescription: %#v", plan.Exercises[0])
	}
	if len(plan.WarmUpItems) != 1 || len(plan.CoolDownItems) != 1 {
		t.Fatal("expected deterministic warm-up and cool-down activities")
	}
	if repository.savedPlan != plan {
		t.Fatal("daily plan was not persisted")
	}
	if repository.schedule.Days[0].DailyPlanID != plan.ID {
		t.Fatal("daily plan was not attached to its schedule day")
	}
	if searcher.criteria.Limit != 4 {
		t.Fatalf("expected four exercises for a multi-group day, got %d", searcher.criteria.Limit)
	}
}

func TestGenerateDailyPlanReturnsExistingPlan(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	existing := domain.NewDailyWorkoutPlan(
		"daily-plan-existing",
		"user-1",
		"roadmap-1",
		"schedule-1",
		startDate,
		nil,
		nil,
		nil,
		startDate,
	)
	repository := newDailyPlanRepository(t, startDate, existing)
	searcher := &exerciseSearcherStub{}
	handler := NewGenerateDailyPlanHandler(
		repository,
		searcher,
		fixedClock{value: startDate},
		&idGeneratorStub{t: t},
	)

	plan, err := handler.Handle(context.Background(), &GenerateDailyPlan{
		UserID:           "user-1",
		WeeklyScheduleID: "schedule-1",
		ScheduledDate:    startDate,
	})
	if err != nil {
		t.Fatalf("get idempotent daily plan: %v", err)
	}
	if plan != existing {
		t.Fatal("expected the existing plan")
	}
	if searcher.calls != 0 || repository.savedPlan != nil {
		t.Fatal("idempotent retry must not search or persist a second plan")
	}
}

func TestGenerateDailyPlanRejectsRestDay(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	repository := newDailyPlanRepository(t, startDate, nil)
	repository.schedule.Days[0].Status = domain.ScheduleDayStatusRest
	handler := NewGenerateDailyPlanHandler(
		repository,
		&exerciseSearcherStub{},
		fixedClock{},
		&idGeneratorStub{t: t},
	)

	_, err := handler.Handle(context.Background(), &GenerateDailyPlan{
		UserID:           "user-1",
		WeeklyScheduleID: "schedule-1",
		ScheduledDate:    startDate,
	})
	if !errors.Is(err, ErrRestDay) {
		t.Fatalf("expected rest-day error, got %v", err)
	}
}

func TestGenerateDailyPlanBlocksInjuryWithoutSafetyMetadata(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	repository := newDailyPlanRepository(t, startDate, nil)
	searcher := &exerciseSearcherStub{}
	handler := NewGenerateDailyPlanHandler(
		repository,
		searcher,
		fixedClock{},
		&idGeneratorStub{t: t},
	)

	_, err := handler.Handle(context.Background(), &GenerateDailyPlan{
		UserID:           "user-1",
		WeeklyScheduleID: "schedule-1",
		ScheduledDate:    startDate,
		NewInjuryAreas:   []string{"shoulder"},
	})
	if !errors.Is(err, ErrInjurySafetyBlock) {
		t.Fatalf("expected injury safety block, got %v", err)
	}
	if searcher.calls != 0 {
		t.Fatal("injury safety block must happen before exercise search")
	}
}

func TestGenerateDailyPlanRequiresMatchingExercise(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	repository := newDailyPlanRepository(t, startDate, nil)
	handler := NewGenerateDailyPlanHandler(
		repository,
		&exerciseSearcherStub{},
		fixedClock{},
		&idGeneratorStub{t: t},
	)

	_, err := handler.Handle(context.Background(), &GenerateDailyPlan{
		UserID:           "user-1",
		WeeklyScheduleID: "schedule-1",
		ScheduledDate:    startDate,
	})
	if !errors.Is(err, ErrNoMatchingExercise) {
		t.Fatalf("expected no-matching-exercise error, got %v", err)
	}
}

type dailyPlanRepositoryStub struct {
	t            *testing.T
	schedule     *domain.WeeklySchedule
	roadmap      *domain.WorkoutRoadmap
	existingPlan *domain.DailyWorkoutPlan
	savedPlan    *domain.DailyWorkoutPlan
}

func newDailyPlanRepository(
	t *testing.T,
	startDate time.Time,
	existingPlan *domain.DailyWorkoutPlan,
) *dailyPlanRepositoryStub {
	t.Helper()
	input := domain.PlanningInput{
		Goal:                domain.PlanningGoalGeneralFitness,
		ExperienceLevel:     domain.ExperienceLevelBeginner,
		TrainingDaysPerWeek: 3,
		PreferredWeekdays:   []time.Weekday{time.Monday, time.Wednesday, time.Friday},
		MaxSessionMinutes:   60,
		EquipmentIDs:        []string{"bodyweight"},
		Timezone:            "Asia/Ho_Chi_Minh",
		StartDate:           startDate,
	}
	roadmap, err := domain.NewWorkoutRoadmap("roadmap-1", "user-1", &input, "rules-v1")
	if err != nil {
		t.Fatalf("create roadmap fixture: %v", err)
	}
	days := []domain.ScheduleDay{
		{
			Date:         startDate,
			Status:       domain.ScheduleDayStatusTraining,
			MuscleGroups: []string{"chest", "triceps"},
		},
	}
	for offset := 1; offset < 7; offset++ {
		days = append(days, domain.ScheduleDay{
			Date:   startDate.AddDate(0, 0, offset),
			Status: domain.ScheduleDayStatusRest,
		})
	}
	schedule, err := domain.NewWeeklySchedule("schedule-1", roadmap.ID, roadmap.UserID, 1, days)
	if err != nil {
		t.Fatalf("create schedule fixture: %v", err)
	}
	return &dailyPlanRepositoryStub{
		t:            t,
		schedule:     schedule,
		roadmap:      roadmap,
		existingPlan: existingPlan,
	}
}

func (r *dailyPlanRepositoryStub) CreateRoadmapWithSchedule(
	context.Context,
	*domain.WorkoutRoadmap,
	*domain.WeeklySchedule,
	[]domain.Event,
) error {
	r.t.Fatal("unexpected CreateRoadmapWithSchedule call")
	return nil
}

func (r *dailyPlanRepositoryStub) SaveSchedule(
	context.Context,
	*domain.WeeklySchedule,
	*domain.Event,
) error {
	r.t.Fatal("unexpected SaveSchedule call")
	return nil
}

func (r *dailyPlanRepositoryStub) FindActiveRoadmapByUser(
	context.Context,
	string,
) (*domain.WorkoutRoadmap, error) {
	r.t.Fatal("unexpected FindActiveRoadmapByUser call")
	return nil, domain.ErrNotFound
}

func (r *dailyPlanRepositoryStub) FindRoadmap(
	context.Context,
	string,
	string,
) (*domain.WorkoutRoadmap, error) {
	return r.roadmap, nil
}

func (r *dailyPlanRepositoryStub) ListRoadmaps(
	context.Context,
	string,
) ([]*domain.WorkoutRoadmap, error) {
	r.t.Fatal("unexpected ListRoadmaps call")
	return nil, nil
}

func (r *dailyPlanRepositoryStub) FindSchedule(
	context.Context,
	string,
	string,
) (*domain.WeeklySchedule, error) {
	return r.schedule, nil
}

func (r *dailyPlanRepositoryStub) FindScheduleByWeek(
	context.Context,
	string,
	int,
) (*domain.WeeklySchedule, error) {
	r.t.Fatal("unexpected FindScheduleByWeek call")
	return nil, domain.ErrNotFound
}

func (r *dailyPlanRepositoryStub) ListSchedules(
	context.Context,
	string,
	string,
) ([]*domain.WeeklySchedule, error) {
	r.t.Fatal("unexpected ListSchedules call")
	return nil, nil
}

func (r *dailyPlanRepositoryStub) SaveDailyPlan(
	_ context.Context,
	schedule *domain.WeeklySchedule,
	plan *domain.DailyWorkoutPlan,
	_ *domain.Event,
) error {
	r.schedule = schedule
	r.savedPlan = plan
	return nil
}

func (r *dailyPlanRepositoryStub) FindDailyPlan(
	context.Context,
	string,
	string,
) (*domain.DailyWorkoutPlan, error) {
	r.t.Fatal("unexpected FindDailyPlan call")
	return nil, domain.ErrNotFound
}

func (r *dailyPlanRepositoryStub) FindDailyPlanByDate(
	context.Context,
	string,
	time.Time,
) (*domain.DailyWorkoutPlan, error) {
	if r.existingPlan != nil {
		return r.existingPlan, nil
	}
	return nil, domain.ErrNotFound
}

type exerciseSearcherStub struct {
	candidates []port.ExerciseCandidate
	criteria   port.ExerciseSearchCriteria
	calls      int
}

func (s *exerciseSearcherStub) Search(
	_ context.Context,
	criteria port.ExerciseSearchCriteria,
) ([]port.ExerciseCandidate, error) {
	s.calls++
	s.criteria = criteria
	return s.candidates, nil
}

type fixedClock struct {
	value time.Time
}

func (c fixedClock) Now() time.Time {
	return c.value
}

type idGeneratorStub struct {
	t   *testing.T
	ids []string
}

func (g *idGeneratorStub) NewID() (string, error) {
	if len(g.ids) == 0 {
		if g.t != nil {
			g.t.Fatal("unexpected id generation")
		}
		return "", errors.New("no stub id available")
	}
	id := g.ids[0]
	g.ids = g.ids[1:]
	return id, nil
}
