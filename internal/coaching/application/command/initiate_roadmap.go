package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

// InitiateRoadmapCommand contains the data needed to create a new roadmap.
type InitiateRoadmapCommand struct {
	UserID             string
	Goals              []string
	RegisteredInjuries []string
	ExperienceLevel    string
}

// InitiateRoadmapHandler orchestrates the creation of a 4-week roadmap,
// the first weekly schedule, and daily workout plans for training days.
type InitiateRoadmapHandler struct {
	roadmapRepo  domain.RoadmapRepository
	scheduleRepo domain.WeeklyScheduleRepository
	planRepo     domain.DailyWorkoutPlanRepository
	exerciseSvc  port.ExerciseQueryService
	planner      port.WorkoutPlanner
	clock        port.Clock
	ids          port.IDGenerator
}

// NewInitiateRoadmapHandler creates a new handler with all dependencies injected.
func NewInitiateRoadmapHandler(
	roadmapRepo domain.RoadmapRepository,
	scheduleRepo domain.WeeklyScheduleRepository,
	planRepo domain.DailyWorkoutPlanRepository,
	exerciseSvc port.ExerciseQueryService,
	planner port.WorkoutPlanner,
	clock port.Clock,
	ids port.IDGenerator,
) *InitiateRoadmapHandler {
	return &InitiateRoadmapHandler{
		roadmapRepo:  roadmapRepo,
		scheduleRepo: scheduleRepo,
		planRepo:     planRepo,
		exerciseSvc:  exerciseSvc,
		planner:      planner,
		clock:        clock,
		ids:          ids,
	}
}

// Handle executes the roadmap initiation flow:
// 1. Idempotency check (active roadmap exists?)
// 2. Create WorkoutRoadmap (4 weeks)
// 3. Generate WeeklySchedule (week 1)
// 4. For each training day: search exercises → plan workout → create daily plan
// 5. Save all in transaction + outbox events
func (h *InitiateRoadmapHandler) Handle(ctx context.Context, cmd *InitiateRoadmapCommand) error {
	now := h.clock.Now()

	// 1. Idempotency: skip if user already has an active roadmap
	existing, err := h.roadmapRepo.FindActiveByUserID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("check existing active roadmap: %w", err)
	}
	if existing != nil {
		log.Printf("INFO: user %s already has active roadmap %s, skipping initiation", cmd.UserID, existing.ID())
		return nil
	}

	// 2. Create WorkoutRoadmap
	roadmapID, err := h.ids.NewID()
	if err != nil {
		return fmt.Errorf("generate roadmap id: %w", err)
	}

	startDate := truncateToDate(now)
	roadmap, err := domain.Initiate(roadmapID, cmd.UserID, startDate, now)
	if err != nil {
		return fmt.Errorf("initiate roadmap: %w", err)
	}

	roadmapEvent, err := h.buildRoadmapEvent(roadmap, now)
	if err != nil {
		return fmt.Errorf("build roadmap event: %w", err)
	}

	if err = h.roadmapRepo.Save(ctx, roadmap, roadmapEvent); err != nil {
		return fmt.Errorf("save roadmap: %w", err)
	}
	log.Printf("INFO: created roadmap %s for user %s", roadmapID, cmd.UserID)

	// 3. Generate WeeklySchedule (week 1)
	scheduleID, err := h.ids.NewID()
	if err != nil {
		return fmt.Errorf("generate schedule id: %w", err)
	}

	schedule, err := domain.GenerateWeeklySchedule(
		scheduleID, roadmapID, cmd.UserID,
		1, startDate,
		cmd.ExperienceLevel,
		now,
	)
	if err != nil {
		return fmt.Errorf("generate weekly schedule: %w", err)
	}

	scheduleEvent, err := h.buildScheduleEvent(schedule, now)
	if err != nil {
		return fmt.Errorf("build schedule event: %w", err)
	}

	if err = h.scheduleRepo.Save(ctx, schedule, scheduleEvent); err != nil {
		return fmt.Errorf("save weekly schedule: %w", err)
	}
	log.Printf("INFO: generated weekly schedule %s (week 1) for roadmap %s", scheduleID, roadmapID)

	// 4. Create DailyWorkoutPlans for each training day
	trainingDays := schedule.TrainingDays()
	if len(trainingDays) == 0 {
		log.Printf("WARN: no training days in schedule %s, skipping plan generation", scheduleID)
		return nil
	}

	plans := make([]*domain.DailyWorkoutPlan, 0, len(trainingDays))
	planEvents := make([]*domain.Event, 0, len(trainingDays))

	for _, day := range trainingDays {
		plan, event, planErr := h.createDailyPlan(ctx, day, schedule, roadmap, cmd, now)
		if planErr != nil {
			return fmt.Errorf("create daily plan for %s: %w", day.Date.Format("2006-01-02"), planErr)
		}

		// Link plan to schedule day
		if linkErr := schedule.SetDailyPlanID(day.Date, plan.ID()); linkErr != nil {
			return fmt.Errorf("link plan to schedule: %w", linkErr)
		}

		plans = append(plans, plan)
		planEvents = append(planEvents, event)
	}

	if err = h.planRepo.SaveBatch(ctx, plans, planEvents); err != nil {
		return fmt.Errorf("save daily plans batch: %w", err)
	}

	// Update schedule with linked plan IDs
	if err = h.scheduleRepo.Save(ctx, schedule, nil); err != nil {
		return fmt.Errorf("update schedule with plan links: %w", err)
	}

	log.Printf("INFO: created %d daily plans for schedule %s", len(plans), scheduleID)
	return nil
}

func (h *InitiateRoadmapHandler) createDailyPlan(
	ctx context.Context,
	day domain.ScheduleDay,
	schedule *domain.WeeklySchedule,
	roadmap *domain.WorkoutRoadmap,
	cmd *InitiateRoadmapCommand,
	now time.Time,
) (*domain.DailyWorkoutPlan, *domain.Event, error) {
	// Search exercises from Exercise module via gRPC
	exercises, err := h.exerciseSvc.SearchExercises(ctx, port.ExerciseSearchFilters{
		TargetMuscleGroups: day.TargetMuscleGroups,
		AvoidInjuryAreas:   cmd.RegisteredInjuries,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("search exercises: %w", err)
	}

	// Ask planner (mock → Gemini) to arrange exercises
	planResult, err := h.planner.PlanWorkout(ctx, port.PlanWorkoutRequest{
		AvailableExercises: exercises,
		TargetMuscleGroups: day.TargetMuscleGroups,
		Goals:              cmd.Goals,
		ExperienceLevel:    cmd.ExperienceLevel,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("plan workout: %w", err)
	}

	// Build prescription from planner result
	prescription := buildPrescription(exercises, planResult)

	planID, err := h.ids.NewID()
	if err != nil {
		return nil, nil, fmt.Errorf("generate plan id: %w", err)
	}

	plan, err := domain.CreateDailyPlan(
		planID, schedule.ID(), roadmap.ID(), cmd.UserID,
		day.Date, prescription, planResult.ReasoningExplanation, now,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create daily plan: %w", err)
	}

	event, err := h.buildPlanEvent(plan, now)
	if err != nil {
		return nil, nil, fmt.Errorf("build plan event: %w", err)
	}

	return plan, event, nil
}

// buildPrescription maps planner result + exercise info into a WorkoutPrescription.
func buildPrescription(exercises []port.ExerciseInfo, result *port.PlanWorkoutResult) domain.WorkoutPrescription {
	exerciseMap := make(map[string]port.ExerciseInfo, len(exercises))
	for _, e := range exercises {
		exerciseMap[e.ID] = e
	}

	mainExercises := make([]domain.PrescribedExercise, 0, len(result.SelectedExerciseIDs))
	for _, id := range result.SelectedExerciseIDs {
		ex, ok := exerciseMap[id]
		if !ok {
			continue
		}
		mainExercises = append(mainExercises, domain.PrescribedExercise{
			ExerciseID:   ex.ID,
			ExerciseName: ex.Name,
			TargetSets:   3, // ponytail: default 3 sets, refine when Gemini planner returns specifics
			TargetReps:   10,
		})
	}

	return domain.WorkoutPrescription{
		MainExercises: mainExercises,
	}
}

// Event builders

type roadmapEventPayload struct {
	RoadmapID string `json:"roadmapId"`
	UserID    string `json:"userId"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type scheduleEventPayload struct {
	WeeklyScheduleID string `json:"weeklyScheduleId"`
	RoadmapID        string `json:"roadmapId"`
	UserID           string `json:"userId"`
	WeekNumber       int    `json:"weekNumber"`
	StartDate        string `json:"startDate"`
	EndDate          string `json:"endDate"`
}

type planEventPayload struct {
	DailyWorkoutPlanID string `json:"dailyWorkoutPlanId"`
	WeeklyScheduleID   string `json:"weeklyScheduleId"`
	RoadmapID          string `json:"roadmapId"`
	UserID             string `json:"userId"`
	ScheduledDate      string `json:"scheduledDate"`
}

func (h *InitiateRoadmapHandler) buildRoadmapEvent(r *domain.WorkoutRoadmap, now time.Time) (*domain.Event, error) {
	return h.buildEvent(domain.EventTypeRoadmapInitiated, r.UserID(), roadmapEventPayload{
		RoadmapID: r.ID(),
		UserID:    r.UserID(),
		StartDate: r.StartDate().Format("2006-01-02"),
		EndDate:   r.EndDate().Format("2006-01-02"),
	}, now)
}

func (h *InitiateRoadmapHandler) buildScheduleEvent(s *domain.WeeklySchedule, now time.Time) (*domain.Event, error) {
	return h.buildEvent(domain.EventTypeWeeklyScheduleGenerated, s.UserID(), scheduleEventPayload{
		WeeklyScheduleID: s.ID(),
		RoadmapID:        s.RoadmapID(),
		UserID:           s.UserID(),
		WeekNumber:       s.WeekNumber(),
		StartDate:        s.StartDate().Format("2006-01-02"),
		EndDate:          s.EndDate().Format("2006-01-02"),
	}, now)
}

func (h *InitiateRoadmapHandler) buildPlanEvent(p *domain.DailyWorkoutPlan, now time.Time) (*domain.Event, error) {
	return h.buildEvent(domain.EventTypeDailyWorkoutPlanGenerated, p.UserID(), planEventPayload{
		DailyWorkoutPlanID: p.ID(),
		WeeklyScheduleID:  p.WeeklyScheduleID(),
		RoadmapID:         p.RoadmapID(),
		UserID:             p.UserID(),
		ScheduledDate:      p.ScheduledDate().Format("2006-01-02"),
	}, now)
}

func (h *InitiateRoadmapHandler) buildEvent(eventType, partitionKey string, payload any, now time.Time) (*domain.Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}

	id, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate event id: %w", err)
	}

	return &domain.Event{
		ID:           id,
		Type:         eventType,
		PartitionKey: partitionKey,
		Payload:      data,
		CreatedAt:    now,
	}, nil
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// ErrInvalidOutboxPayload is returned when an event payload cannot be serialized.
var ErrInvalidOutboxPayload = errors.New("invalid outbox payload")
