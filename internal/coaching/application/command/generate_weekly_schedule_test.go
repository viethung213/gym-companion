package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestGenerateWeeklyScheduleUsesPreviousScheduleAsIdempotencyCursor(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	base := newDailyPlanRepository(t, startDate, nil)
	days, err := (domain.SchedulePlanner{}).PlanWeek(&base.roadmap.Input, 2)
	if err != nil {
		t.Fatalf("plan second week fixture: %v", err)
	}
	existing, err := domain.NewWeeklySchedule(
		"schedule-2",
		base.roadmap.ID,
		base.roadmap.UserID,
		2,
		days,
	)
	if err != nil {
		t.Fatalf("create second week fixture: %v", err)
	}
	repository := &weeklyScheduleRepositoryStub{
		dailyPlanRepositoryStub: base,
		existingNext:            existing,
	}
	handler := NewGenerateWeeklyScheduleHandler(
		repository,
		fixedClock{value: startDate},
		&idGeneratorStub{t: t},
	)

	schedule, err := handler.Handle(context.Background(), GenerateWeeklySchedule{
		UserID:                   "user-1",
		RoadmapID:                "roadmap-1",
		PreviousWeeklyScheduleID: "schedule-1",
	})
	if err != nil {
		t.Fatalf("retry generate schedule: %v", err)
	}
	if schedule != existing {
		t.Fatal("retry must return the schedule already generated from the same previous week")
	}
	if repository.saved != nil {
		t.Fatal("retry must not persist another schedule")
	}
}

func TestGenerateWeeklyScheduleCreatesWeekAfterPreviousSchedule(t *testing.T) {
	t.Parallel()

	startDate := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	repository := &weeklyScheduleRepositoryStub{
		dailyPlanRepositoryStub: newDailyPlanRepository(t, startDate, nil),
	}
	handler := NewGenerateWeeklyScheduleHandler(
		repository,
		fixedClock{value: startDate},
		&idGeneratorStub{ids: []string{"schedule-2"}},
	)

	schedule, err := handler.Handle(context.Background(), GenerateWeeklySchedule{
		UserID:                   "user-1",
		RoadmapID:                "roadmap-1",
		PreviousWeeklyScheduleID: "schedule-1",
	})
	if err != nil {
		t.Fatalf("generate second week: %v", err)
	}
	if schedule.WeekNumber != 2 || repository.saved != schedule {
		t.Fatalf("unexpected generated schedule: %#v", schedule)
	}
}

type weeklyScheduleRepositoryStub struct {
	*dailyPlanRepositoryStub
	existingNext *domain.WeeklySchedule
	saved        *domain.WeeklySchedule
}

func (r *weeklyScheduleRepositoryStub) FindSchedule(
	_ context.Context,
	_ string,
	scheduleID string,
) (*domain.WeeklySchedule, error) {
	if scheduleID == r.schedule.ID {
		return r.schedule, nil
	}
	return nil, domain.ErrNotFound
}

func (r *weeklyScheduleRepositoryStub) FindScheduleByWeek(
	_ context.Context,
	_ string,
	weekNumber int,
) (*domain.WeeklySchedule, error) {
	if r.existingNext != nil && r.existingNext.WeekNumber == weekNumber {
		return r.existingNext, nil
	}
	return nil, domain.ErrNotFound
}

func (r *weeklyScheduleRepositoryStub) SaveSchedule(
	_ context.Context,
	schedule *domain.WeeklySchedule,
	_ *domain.Event,
) error {
	r.saved = schedule
	return nil
}
