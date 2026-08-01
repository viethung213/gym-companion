package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// CreateAdhocSessionCommand is the input for Flow 2.1 (ad-hoc session creation).
type CreateAdhocSessionCommand struct {
	UserID      string
	ExerciseIDs []string
}

// CreateAdhocSessionResult wraps the created session plan.
type CreateAdhocSessionResult struct {
	SessionPlan *roadmap.SessionPlan
}

// CreateAdhocSessionHandler creates an ad-hoc session plan and adds it to
// today's DayPlan (or the last day if today has no plan).
type CreateAdhocSessionHandler struct {
	tx        port.TransactionManager
	repo      port.RoadmapRepository
	exercises port.ExerciseCatalogReader
	idgen     port.IDGenerator
	clock     port.Clock
}

// NewCreateAdhocSessionHandler wires the handler.
func NewCreateAdhocSessionHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	exercises port.ExerciseCatalogReader,
	idgen port.IDGenerator,
	clock port.Clock,
) *CreateAdhocSessionHandler {
	return &CreateAdhocSessionHandler{
		tx:        tx,
		repo:      repo,
		exercises: exercises,
		idgen:     idgen,
		clock:     clock,
	}
}

// Handle executes the ad-hoc session creation.
func (h *CreateAdhocSessionHandler) Handle(ctx context.Context, cmd CreateAdhocSessionCommand) (*CreateAdhocSessionResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("user_id is required")
	}

	if len(cmd.ExerciseIDs) == 0 {
		return nil, errors.New("at least one exercise_id is required")
	}

	var out *CreateAdhocSessionResult

	err := h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm, err := h.repo.FindActiveByUser(txCtx, cmd.UserID)
		if err != nil {
			if errors.Is(err, roadmap.ErrRoadmapNotFound) {
				return fmt.Errorf("no active roadmap for user: %w", err)
			}
			return fmt.Errorf("find active roadmap: %w", err)
		}

		// Find the DayPlan for today (or the last day in the roadmap).
		today := h.clock.Now().Truncate(24 * time.Hour)
		dayPlan := findOrFallbackDayPlan(rm, today)
		if dayPlan == nil {
			return errors.New("no day plan found for today or as fallback")
		}

		// Resolve exercise details.
		exercises := make([]port.Exercise, 0, len(cmd.ExerciseIDs))
		for _, exID := range cmd.ExerciseIDs {
			ex, getErr := h.exercises.GetByID(txCtx, exID)
			if getErr != nil {
				return fmt.Errorf("get exercise %s: %w", exID, getErr)
			}
			exercises = append(exercises, ex)
		}

		// Build a minimal prescription from the exercises.
		// For ad-hoc sessions, all exercises are main exercises (no warm-up/cool-down).
		prescription := roadmap.WorkoutPrescription{
			MainExercises: exercisesToPrescribed(exercises),
		}

		// Collect target muscle groups.
		muscleGroups := uniqueMuscleGroups(exercises)

		now := h.clock.Now()

		// Create the SessionPlan with USER_ADHOC source.
		sessionPlan, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
			SessionPlanID:      h.idgen.NewID(),
			DayPlanID:          dayPlan.ID(),
			WeekPlanID:         dayPlan.Info().WeekPlanID,
			RoadmapID:          rm.ID(),
			UserID:             cmd.UserID,
			ScheduledDate:      today,
			SlotTime:           "",
			Source:             roadmap.SessionPlanSourceAdHoc,
			TargetMuscleGroups: muscleGroups,
			Prescription:       prescription,
			Reasoning:          "ad-hoc session created by user",
		}, now)

		if err != nil {
			return fmt.Errorf("create session plan: %w", err)
		}

		// Add the session to the day plan.
		if err := dayPlan.AddSession(sessionPlan); err != nil {
			return fmt.Errorf("add session to day plan: %w", err)
		}

		// Persist the updated roadmap.
		if err := h.repo.Save(txCtx, rm); err != nil {
			return fmt.Errorf("save roadmap: %w", err)
		}

		out = &CreateAdhocSessionResult{SessionPlan: sessionPlan}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return out, nil
}

// findOrFallbackDayPlan finds a DayPlan for the given date.
// If no exact match, returns the last day in the roadmap.
func findOrFallbackDayPlan(rm *roadmap.Roadmap, date time.Time) *roadmap.DayPlan {
	var lastDay *roadmap.DayPlan

	for _, w := range rm.Weeks() {
		for _, d := range w.Days() {
			lastDay = d
			if d.Info().ScheduledDate.Truncate(24*time.Hour) == date {
				return d
			}
		}
	}

	return lastDay
}

// exercisesToPrescribed converts exercise details to PrescribedExercise items.
func exercisesToPrescribed(exercises []port.Exercise) []roadmap.PrescribedExercise {
	out := make([]roadmap.PrescribedExercise, len(exercises))
	for i, ex := range exercises {
		out[i] = roadmap.PrescribedExercise{
			ExerciseID:      ex.ExerciseID,
			ExerciseName:    ex.Name,
			TargetSets:      3,
			TargetReps:      8,
			TargetWeight:    0,
			DurationSeconds: 0,
			Notes:           "",
			RestSetSec:      90,
			RestExerciseSec: 120,
			TargetRPE:       7.0,
		}
	}
	return out
}

// uniqueMuscleGroups extracts and deduplicates muscle groups from exercises.
func uniqueMuscleGroups(exercises []port.Exercise) []string {
	seen := make(map[string]bool)
	var out []string
	for _, ex := range exercises {
		if !seen[ex.MuscleGroup] {
			seen[ex.MuscleGroup] = true
			out = append(out, ex.MuscleGroup)
		}
	}
	return out
}
