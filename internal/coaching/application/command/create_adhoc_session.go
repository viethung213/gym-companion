package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// ErrExerciseExcludedByInjury is returned when the athlete picks an exercise
// an active injury rules out (BR-AC-09 applies to ad-hoc sessions too).
var ErrExerciseExcludedByInjury = errors.New("exercise is excluded by an active injury")

// adHocTargetRPE is the intensity of a user-built session line; an injury cap
// lowers it.
const adHocTargetRPE = 7.0

// CreateAdhocSessionCommand is the input for Flow 2.1 (ad-hoc session creation).
type CreateAdhocSessionCommand struct {
	UserID      string
	ExerciseIDs []string
}

// CreateAdhocSessionResult wraps the created session plan.
type CreateAdhocSessionResult struct {
	SessionPlan *roadmap.SessionPlan
}

// CreateAdhocSessionHandler adds a user-built session to today's DayPlan of the
// active roadmap, today being the athlete's local date.
type CreateAdhocSessionHandler struct {
	tx        port.TransactionManager
	repo      port.RoadmapRepository
	exercises port.ExerciseCatalogReader
	profiles  port.UserProfileReader
	idgen     port.IDGenerator
	clock     port.Clock
	// rollover, when set, moves the athlete to their next block when the
	// active roadmap has ended, so the session lands in the right one.
	rollover roadmapRollover
}

// roadmapRollover is the RollOverHandler contract.
type roadmapRollover interface {
	Handle(ctx context.Context, userID string) (*RollOverResult, error)
}

// errRoadmapEnded marks a session asked for after the active roadmap ended.
var errRoadmapEnded = fmt.Errorf("active roadmap has ended: %w", roadmap.ErrDateOutsideRoadmap)

// WithRollover enables moving to the next block when the active one ended.
func (h *CreateAdhocSessionHandler) WithRollover(r roadmapRollover) *CreateAdhocSessionHandler {
	h.rollover = r
	return h
}

// NewCreateAdhocSessionHandler wires the handler.
func NewCreateAdhocSessionHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	exercises port.ExerciseCatalogReader,
	profiles port.UserProfileReader,
	idgen port.IDGenerator,
	clock port.Clock,
) *CreateAdhocSessionHandler {
	return &CreateAdhocSessionHandler{
		tx:        tx,
		repo:      repo,
		exercises: exercises,
		profiles:  profiles,
		idgen:     idgen,
		clock:     clock,
	}
}

// Handle executes the ad-hoc session creation. An exercise an active injury
// excludes is refused; one it only limits is kept at the injury's RPE cap.
func (h *CreateAdhocSessionHandler) Handle(ctx context.Context, cmd CreateAdhocSessionCommand) (*CreateAdhocSessionResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("user_id is required")
	}

	if len(cmd.ExerciseIDs) == 0 {
		return nil, errors.New("at least one exercise_id is required")
	}

	// Remote reads stay outside the transaction.
	profile, err := h.profiles.GetProfile(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load profile for ad-hoc session: %w", err)
	}
	exercises := make([]port.Exercise, 0, len(cmd.ExerciseIDs))
	for _, exID := range cmd.ExerciseIDs {
		ex, getErr := h.exercises.GetByID(ctx, exID)
		if getErr != nil {
			return nil, fmt.Errorf("get exercise %s: %w", exID, getErr)
		}
		exercises = append(exercises, ex)
	}

	athlete := profile.Athlete()
	mains, err := adHocLines(exercises, athlete.Injuries)
	if err != nil {
		return nil, err
	}

	now := h.clock.Now()
	today := roadmap.CalendarDate(now, profile.Zone())

	out, err := h.place(ctx, cmd.UserID, today, now, mains, exercises)
	if errors.Is(err, errRoadmapEnded) && h.rollover != nil {
		if _, rollErr := h.rollover.Handle(ctx, cmd.UserID); rollErr != nil {
			return nil, rollErr
		}
		out, err = h.place(ctx, cmd.UserID, today, now, mains, exercises)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// place adds the session to today's DayPlan of the user's active roadmap.
func (h *CreateAdhocSessionHandler) place(
	ctx context.Context, userID string, today, now time.Time,
	mains []roadmap.PrescribedExercise, exercises []port.Exercise,
) (*CreateAdhocSessionResult, error) {
	var out *CreateAdhocSessionResult
	err := h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm, findErr := h.repo.FindActiveByUser(txCtx, userID)
		if findErr != nil {
			if errors.Is(findErr, roadmap.ErrRoadmapNotFound) {
				return fmt.Errorf("no active roadmap for user: %w", findErr)
			}
			return fmt.Errorf("find active roadmap: %w", findErr)
		}

		// The session belongs to today; a roadmap that does not cover today
		// cannot take it (it used to land on the roadmap's last day instead).
		dayPlan, dayErr := rm.DayOn(today, h.idgen.NewID)
		if dayErr != nil {
			if errors.Is(dayErr, roadmap.ErrDateOutsideRoadmap) && !today.Before(roadmap.CalendarDate(rm.Info().EndDate, time.UTC)) {
				return errRoadmapEnded
			}
			return fmt.Errorf("place ad-hoc session: %w", dayErr)
		}

		sessionPlan, newErr := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
			SessionPlanID:      h.idgen.NewID(),
			DayPlanID:          dayPlan.ID(),
			WeekPlanID:         dayPlan.Info().WeekPlanID,
			RoadmapID:          rm.ID(),
			UserID:             userID,
			ScheduledDate:      today,
			SlotTime:           "",
			Source:             roadmap.SessionPlanSourceAdHoc,
			TargetMuscleGroups: uniqueMuscleGroups(exercises),
			Prescription:       roadmap.WorkoutPrescription{MainExercises: mains},
			Reasoning:          "ad-hoc session created by user",
		}, now)
		if newErr != nil {
			return fmt.Errorf("create session plan: %w", newErr)
		}

		if addErr := dayPlan.AddSession(sessionPlan); addErr != nil {
			return fmt.Errorf("add session to day plan: %w", addErr)
		}

		if saveErr := h.repo.Save(txCtx, rm); saveErr != nil {
			return fmt.Errorf("save roadmap: %w", saveErr)
		}

		out = &CreateAdhocSessionResult{SessionPlan: sessionPlan}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// adHocLines builds the main exercises of a user-built session. Every exercise
// is held to the athlete's injuries: excluded ones fail the request, limited
// ones are prescribed at their RPE cap with the reason in Notes.
func adHocLines(exercises []port.Exercise, injuries []service.Injury) ([]roadmap.PrescribedExercise, error) {
	out := make([]roadmap.PrescribedExercise, len(exercises))
	for i := range exercises {
		ex := &exercises[i]
		line := roadmap.PrescribedExercise{
			ExerciseID:      ex.ExerciseID,
			ExerciseName:    ex.Name,
			TargetSets:      3,
			TargetReps:      8,
			RestSetSec:      90,
			RestExerciseSec: 120,
			TargetRPE:       adHocTargetRPE,
		}
		if len(injuries) > 0 {
			traits := ex.Traits()
			verdict := service.EvaluateInjuries(&traits, injuries)
			if verdict.Excluded {
				return nil, fmt.Errorf("%w: %s (%s)", ErrExerciseExcludedByInjury, ex.Name, verdict.Reason())
			}
			if verdict.MaxRPE > 0 {
				line.TargetRPE = float32(min(adHocTargetRPE, verdict.MaxRPE))
				line.Notes = "Injury caution: " + verdict.Reason()
			}
		}
		out[i] = line
	}
	return out, nil
}

// uniqueMuscleGroups lists the exercises' primary MuscleGroup codes once each.
func uniqueMuscleGroups(exercises []port.Exercise) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(exercises))
	for i := range exercises {
		mg := string(exercises[i].PrimaryMuscle)
		if !seen[mg] {
			seen[mg] = true
			out = append(out, mg)
		}
	}
	return out
}
