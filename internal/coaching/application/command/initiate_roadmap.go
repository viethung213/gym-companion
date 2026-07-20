package command

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	coachingevent "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/event"
	datepb "google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var errActiveRoadmapExists = errors.New("active roadmap already exists")

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
	unitOfWork   port.UnitOfWork
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
	unitOfWork port.UnitOfWork,
) *InitiateRoadmapHandler {
	return &InitiateRoadmapHandler{
		roadmapRepo:  roadmapRepo,
		scheduleRepo: scheduleRepo,
		planRepo:     planRepo,
		exerciseSvc:  exerciseSvc,
		planner:      planner,
		clock:        clock,
		ids:          ids,
		unitOfWork:   unitOfWork,
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

	existing, err := h.roadmapRepo.FindActiveByUserID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("check existing active roadmap: %w", err)
	}
	if existing != nil {
		log.Printf(
			"INFO: user %s already has active roadmap %s, skipping initiation",
			cmd.UserID,
			existing.ID(),
		)
		return nil
	}

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

	trainingDays := schedule.TrainingDays()
	plans := make([]*domain.DailyWorkoutPlan, 0, len(trainingDays))
	planEvents := make([]*domain.Event, 0, len(trainingDays))

	for i := range trainingDays {
		day := &trainingDays[i]
		plan, event, planErr := h.createDailyPlan(
			ctx,
			day,
			schedule,
			roadmap,
			cmd,
			now,
		)
		if planErr != nil {
			return fmt.Errorf(
				"create daily plan for %s: %w",
				day.Date.Format("2006-01-02"),
				planErr,
			)
		}

		if linkErr := schedule.SetDailyPlanID(day.Date, plan.ID()); linkErr != nil {
			return fmt.Errorf("link plan to schedule: %w", linkErr)
		}

		plans = append(plans, plan)
		planEvents = append(planEvents, event)
	}

	err = h.unitOfWork.WithinTransaction(ctx, func(txCtx context.Context) error {
		active, findErr := h.roadmapRepo.FindActiveByUserID(txCtx, cmd.UserID)
		if findErr != nil {
			return fmt.Errorf("recheck active roadmap: %w", findErr)
		}
		if active != nil {
			return errActiveRoadmapExists
		}

		if saveErr := h.roadmapRepo.Save(txCtx, roadmap, roadmapEvent); saveErr != nil {
			return fmt.Errorf("save roadmap: %w", saveErr)
		}
		if saveErr := h.scheduleRepo.Save(txCtx, schedule, scheduleEvent); saveErr != nil {
			return fmt.Errorf("save weekly schedule: %w", saveErr)
		}
		if saveErr := h.planRepo.SaveBatch(txCtx, plans, planEvents); saveErr != nil {
			return fmt.Errorf("save daily plans batch: %w", saveErr)
		}

		return nil
	})
	if errors.Is(err, errActiveRoadmapExists) ||
		errors.Is(err, domain.ErrRoadmapAlreadyActive) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("persist roadmap initiation: %w", err)
	}

	log.Printf("INFO: created %d daily plans for schedule %s", len(plans), scheduleID)
	return nil
}

func (h *InitiateRoadmapHandler) createDailyPlan(
	ctx context.Context,
	day *domain.ScheduleDay,
	schedule *domain.WeeklySchedule,
	roadmap *domain.WorkoutRoadmap,
	cmd *InitiateRoadmapCommand,
	now time.Time,
) (*domain.DailyWorkoutPlan, *domain.Event, error) {
	// Search exercises from Exercise module via gRPC
	exercises, err := h.exerciseSvc.SearchExercises(ctx, &port.ExerciseSearchFilters{
		TargetMuscleGroups: day.TargetMuscleGroups,
		AvoidInjuryAreas:   cmd.RegisteredInjuries,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("search exercises: %w", err)
	}
	if len(exercises) == 0 {
		return nil, nil, errors.New("search exercises: no safe exercises available")
	}

	// Ask planner (mock → Gemini) to arrange exercises
	planResult, err := h.planner.PlanWorkout(ctx, &port.PlanWorkoutRequest{
		AvailableExercises: exercises,
		TargetMuscleGroups: day.TargetMuscleGroups,
		Goals:              cmd.Goals,
		ExperienceLevel:    cmd.ExperienceLevel,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("plan workout: %w", err)
	}

	// Build prescription from planner result
	prescription := buildPrescription(exercises, planResult, cmd.ExperienceLevel)
	if len(prescription.MainExercises) == 0 {
		return nil, nil, errors.New("plan workout: planner selected no valid exercises")
	}

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

func buildPrescription(
	exercises []port.ExerciseInfo,
	result *port.PlanWorkoutResult,
	experienceLevel string,
) domain.WorkoutPrescription {
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

		spec := domain.ResolvePrescriptionSpec(experienceLevel, ex.Category)

		mainExercises = append(mainExercises, domain.PrescribedExercise{
			ExerciseID:   ex.ID,
			ExerciseName: ex.Name,
			TargetSets:   spec.TargetSets,
			TargetReps:   spec.TargetReps,
		})
	}

	return domain.WorkoutPrescription{
		MainExercises: mainExercises,
	}
}

func (h *InitiateRoadmapHandler) buildRoadmapEvent(
	roadmap *domain.WorkoutRoadmap,
	now time.Time,
) (*domain.Event, error) {
	payload := &coachingevent.RoadmapInitiated{
		RoadmapId:   roadmap.ID(),
		StartDate:   toProtoDate(roadmap.StartDate()),
		EndDate:     toProtoDate(roadmap.EndDate()),
		InitiatedAt: timestamppb.New(now),
	}

	return h.buildEvent(
		domain.EventTypeRoadmapInitiated,
		roadmap.UserID(),
		payload,
		now,
	)
}

func (h *InitiateRoadmapHandler) buildScheduleEvent(
	schedule *domain.WeeklySchedule,
	now time.Time,
) (*domain.Event, error) {
	payload := &coachingevent.WeeklyScheduleGenerated{
		WeeklyScheduleId: schedule.ID(),
		RoadmapId:        schedule.RoadmapID(),
		WeekNumber:       int32(schedule.WeekNumber()),
		StartDate:        toProtoDate(schedule.StartDate()),
		EndDate:          toProtoDate(schedule.EndDate()),
		GeneratedAt:      timestamppb.New(now),
	}

	return h.buildEvent(
		domain.EventTypeWeeklyScheduleGenerated,
		schedule.UserID(),
		payload,
		now,
	)
}

func (h *InitiateRoadmapHandler) buildPlanEvent(
	plan *domain.DailyWorkoutPlan,
	now time.Time,
) (*domain.Event, error) {
	payload := &coachingevent.DailyWorkoutPlanGenerated{
		DailyWorkoutPlanId: plan.ID(),
		WeeklyScheduleId:   plan.WeeklyScheduleID(),
		RoadmapId:          plan.RoadmapID(),
		ScheduledDate:      toProtoDate(plan.ScheduledDate()),
		GeneratedAt:        timestamppb.New(now),
	}

	return h.buildEvent(
		domain.EventTypeDailyWorkoutPlanGenerated,
		plan.UserID(),
		payload,
		now,
	)
}

func (h *InitiateRoadmapHandler) buildEvent(
	eventType, partitionKey string,
	payload proto.Message,
	now time.Time,
) (*domain.Event, error) {
	data, err := protojson.Marshal(payload)
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

func toProtoDate(value time.Time) *datepb.Date {
	return &datepb.Date{
		Year:  int32(value.Year()),
		Month: int32(value.Month()),
		Day:   int32(value.Day()),
	}
}
