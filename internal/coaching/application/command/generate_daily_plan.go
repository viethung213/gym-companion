package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

var (
	ErrRestDay           = errors.New("daily plan cannot be generated for a rest day")
	ErrInjurySafetyBlock = errors.New(
		"daily plan blocked because injury safety metadata is unavailable",
	)
	ErrNoMatchingExercise = errors.New("no matching exercises were found")
)

type GenerateDailyPlan struct {
	UserID           string
	WeeklyScheduleID string
	ScheduledDate    time.Time
	NewInjuryAreas   []string
}

type GenerateDailyPlanHandler struct {
	repository port.Repository
	searcher   port.ExerciseSearcher
	clock      port.Clock
	ids        port.IDGenerator
	planner    domain.PrescriptionPlanner
}

func NewGenerateDailyPlanHandler(
	repository port.Repository,
	searcher port.ExerciseSearcher,
	clock port.Clock,
	ids port.IDGenerator,
) *GenerateDailyPlanHandler {
	return &GenerateDailyPlanHandler{
		repository: repository,
		searcher:   searcher,
		clock:      clock,
		ids:        ids,
	}
}

func (h *GenerateDailyPlanHandler) Handle(
	ctx context.Context,
	command *GenerateDailyPlan,
) (*domain.DailyWorkoutPlan, error) {
	schedule, err := h.repository.FindSchedule(ctx, command.UserID, command.WeeklyScheduleID)
	if err != nil {
		return nil, fmt.Errorf("find schedule: %w", err)
	}
	day, err := findScheduleDay(schedule, command.ScheduledDate)
	if err != nil {
		return nil, err
	}
	if day.Status == domain.ScheduleDayStatusRest {
		return nil, ErrRestDay
	}
	roadmap, err := h.repository.FindRoadmap(ctx, command.UserID, schedule.RoadmapID)
	if err != nil {
		return nil, fmt.Errorf("find roadmap: %w", err)
	}
	if len(command.NewInjuryAreas) > 0 || len(roadmap.Input.ActiveInjuryAreas) > 0 {
		return nil, ErrInjurySafetyBlock
	}
	if existing, findErr := h.repository.FindDailyPlanByDate(
		ctx,
		schedule.ID,
		command.ScheduledDate,
	); findErr == nil {
		return existing, nil
	} else if !errors.Is(findErr, domain.ErrNotFound) {
		return nil, fmt.Errorf("find daily plan: %w", findErr)
	}

	candidates, err := h.searcher.Search(ctx, port.ExerciseSearchCriteria{
		MuscleGroupIDs: day.MuscleGroups,
		EquipmentIDs:   roadmap.Input.EquipmentIDs,
		Difficulty:     string(roadmap.Input.ExperienceLevel),
		Limit:          exerciseLimit(day),
	})
	if err != nil {
		return nil, fmt.Errorf("search exercises: %w", err)
	}
	if len(candidates) == 0 {
		return nil, ErrNoMatchingExercise
	}
	exerciseOptions := make([]domain.ExerciseOption, 0, len(candidates))
	for _, candidate := range candidates {
		exerciseOptions = append(exerciseOptions, domain.ExerciseOption{
			ID:                 candidate.ID,
			Name:               candidate.Name,
			DefaultRestSeconds: candidate.DefaultRestSeconds,
		})
	}
	exercises := h.planner.Plan(exerciseOptions, roadmap.Input.ExperienceLevel)
	warmUpItems, coolDownItems := h.planner.PlanSessionActivities(roadmap.Input.MaxSessionMinutes)
	planID, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate daily plan id: %w", err)
	}
	generatedAt := h.clock.Now()
	plan := domain.NewDailyWorkoutPlan(
		planID,
		command.UserID,
		schedule.RoadmapID,
		schedule.ID,
		command.ScheduledDate,
		exercises,
		warmUpItems,
		coolDownItems,
		generatedAt,
	)
	if err := schedule.AttachDailyPlan(command.ScheduledDate, plan.ID); err != nil {
		return nil, fmt.Errorf("attach daily plan: %w", err)
	}
	event := newEvent(
		plan.ID,
		command.UserID,
		"contracts.coaching.coachingService.v1.dailyWorkoutPlanGenerated",
		generatedAt,
	)
	if err := h.repository.SaveDailyPlan(ctx, schedule, plan, &event); err != nil {
		return nil, fmt.Errorf("persist daily plan: %w", err)
	}
	return plan, nil
}

func findScheduleDay(
	schedule *domain.WeeklySchedule,
	scheduledDate time.Time,
) (*domain.ScheduleDay, error) {
	for index := range schedule.Days {
		if sameDate(schedule.Days[index].Date, scheduledDate) {
			return &schedule.Days[index], nil
		}
	}
	return nil, domain.ErrNotFound
}

func sameDate(left, right time.Time) bool {
	leftYear, leftMonth, leftDay := left.Date()
	rightYear, rightMonth, rightDay := right.Date()
	return leftYear == rightYear && leftMonth == rightMonth && leftDay == rightDay
}

func exerciseLimit(day *domain.ScheduleDay) int {
	if len(day.MuscleGroups) > 1 {
		return 4
	}
	return 3
}
