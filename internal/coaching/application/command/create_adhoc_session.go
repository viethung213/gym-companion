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

// ErrInvalidAdhocPrescription is returned for a built session line that
// breaks a hard limit (BR-AC-13).
var ErrInvalidAdhocPrescription = errors.New("invalid ad-hoc prescription")

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
	// Prescription, when it has main work, is the session as the athlete
	// built it and takes the place of ExerciseIDs.
	Prescription *roadmap.WorkoutPrescription
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
	// starter, when set, starts a fresh roadmap when the roll-over leaves
	// the athlete without one and asks for it (RollOverResult.StartFresh).
	starter roadmapStarter
}

// roadmapRollover is the RollOverHandler contract.
type roadmapRollover interface {
	Handle(ctx context.Context, userID string) (*RollOverResult, error)
}

// roadmapStarter is the InitiateRoadmapHandler contract.
type roadmapStarter interface {
	Handle(ctx context.Context, cmd InitiateRoadmapCommand) (*InitiateRoadmapResult, error)
}

// errRoadmapEnded marks a session asked for after the active roadmap ended.
var errRoadmapEnded = fmt.Errorf("active roadmap has ended: %w", roadmap.ErrDateOutsideRoadmap)

// WithRollover enables moving to the next block when the active one ended.
func (h *CreateAdhocSessionHandler) WithRollover(r roadmapRollover) *CreateAdhocSessionHandler {
	h.rollover = r
	return h
}

// WithFreshStart starts a new roadmap when the roll-over asks for one, as the
// lifecycle worker would: the worker no longer sees a roadmap rolled over here.
func (h *CreateAdhocSessionHandler) WithFreshStart(s roadmapStarter) *CreateAdhocSessionHandler {
	h.starter = s
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

	built := cmd.Prescription != nil && len(cmd.Prescription.MainExercises) > 0
	if !built && len(cmd.ExerciseIDs) == 0 {
		return nil, errors.New("at least one exercise_id is required")
	}

	// Remote reads stay outside the transaction.
	profile, err := h.profiles.GetProfile(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load profile for ad-hoc session: %w", err)
	}
	injuries := profile.Athlete().Injuries

	var prescription roadmap.WorkoutPrescription
	var mainExercises []port.Exercise
	if built {
		prescription, mainExercises, err = h.builtPrescription(ctx, cmd.Prescription, injuries)
	} else {
		prescription, mainExercises, err = h.defaultPrescription(ctx, cmd.ExerciseIDs, injuries)
	}
	if err != nil {
		return nil, err
	}

	now := h.clock.Now()
	out, err := h.place(ctx, cmd.UserID, now, prescription, mainExercises)
	if errors.Is(err, errRoadmapEnded) && h.rollover != nil {
		if rollErr := h.rollOver(ctx, cmd.UserID); rollErr != nil {
			return nil, rollErr
		}
		out, err = h.place(ctx, cmd.UserID, now, prescription, mainExercises)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// rollOver completes the ended roadmap and, when that leaves the athlete
// without one and a fresh one is due, starts it.
func (h *CreateAdhocSessionHandler) rollOver(ctx context.Context, userID string) error {
	res, err := h.rollover.Handle(ctx, userID)
	if err != nil {
		return err
	}
	if !res.StartFresh || h.starter == nil {
		return nil
	}
	if _, err := h.starter.Handle(ctx, InitiateRoadmapCommand{UserID: userID}); err != nil &&
		!errors.Is(err, roadmap.ErrActiveRoadmapExists) {
		return fmt.Errorf("start a fresh roadmap: %w", err)
	}
	return nil
}

// place adds the session to today's DayPlan of the user's active roadmap.
// "Today" and "the roadmap has ended" are both read in the roadmap's zone,
// the zone its sessions are dated in and the roll-over decides by, so a
// session refused as "after the end" is always one the roll-over moves on.
func (h *CreateAdhocSessionHandler) place(
	ctx context.Context, userID string, now time.Time,
	prescription roadmap.WorkoutPrescription, mainExercises []port.Exercise,
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
		today := roadmap.CalendarDate(now, rm.Location())
		dayPlan, dayErr := rm.DayOn(today, h.idgen.NewID)
		if dayErr != nil {
			if errors.Is(dayErr, roadmap.ErrDateOutsideRoadmap) && !now.Before(rm.EndsAt()) {
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
			TargetMuscleGroups: uniqueMuscleGroups(mainExercises),
			Prescription:       prescription,
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

// defaultPrescription is the legacy ad-hoc session: each exercise a default
// main line.
func (h *CreateAdhocSessionHandler) defaultPrescription(
	ctx context.Context, ids []string, injuries []service.Injury,
) (roadmap.WorkoutPrescription, []port.Exercise, error) {
	exercises, err := h.lookUp(ctx, ids)
	if err != nil {
		return roadmap.WorkoutPrescription{}, nil, err
	}
	mains, err := adHocLines(exercises, injuries)
	if err != nil {
		return roadmap.WorkoutPrescription{}, nil, err
	}
	return roadmap.WorkoutPrescription{MainExercises: mains}, exercises, nil
}

// builtPrescription keeps the session as the athlete built it, warm-ups and
// cool-downs included, and holds every line to the catalog, the athlete's
// injuries and the hard limits: an excluded exercise or a line past a limit
// fails the request, and a limited one is capped at its injury RPE. Missing
// numbers get the defaults of a user-built line.
func (h *CreateAdhocSessionHandler) builtPrescription(
	ctx context.Context, in *roadmap.WorkoutPrescription, injuries []service.Injury,
) (roadmap.WorkoutPrescription, []port.Exercise, error) {
	var mainExercises []port.Exercise
	fit := func(lines []roadmap.PrescribedExercise, main bool) ([]roadmap.PrescribedExercise, error) {
		out := make([]roadmap.PrescribedExercise, 0, len(lines))
		for i := range lines {
			line := lines[i]
			found, err := h.lookUp(ctx, []string{line.ExerciseID})
			if err != nil {
				return nil, err
			}
			ex := &found[0]
			line.ExerciseName = ex.Name
			if issue := line.LimitIssue(); issue != "" {
				return nil, fmt.Errorf("%w: %s: %s", ErrInvalidAdhocPrescription, ex.Name, issue)
			}
			fillAdHocDefaults(&line, main, ex.IsTimed())
			if len(injuries) > 0 {
				traits := ex.Traits()
				verdict := service.EvaluateInjuries(&traits, injuries)
				if verdict.Excluded {
					return nil, fmt.Errorf("%w: %s (%s)", ErrExerciseExcludedByInjury, ex.Name, verdict.Reason())
				}
				if verdict.MaxRPE > 0 && float64(line.TargetRPE) > verdict.MaxRPE {
					line.TargetRPE = float32(verdict.MaxRPE)
					line.Notes = "Injury caution: " + verdict.Reason()
				}
			}
			if main {
				mainExercises = append(mainExercises, *ex)
			}
			out = append(out, line)
		}
		return out, nil
	}

	var out roadmap.WorkoutPrescription
	var err error
	if out.WarmUps, err = fit(in.WarmUps, false); err != nil {
		return out, nil, err
	}
	if out.MainExercises, err = fit(in.MainExercises, true); err != nil {
		return out, nil, err
	}
	if out.CoolDowns, err = fit(in.CoolDowns, false); err != nil {
		return out, nil, err
	}
	return out, mainExercises, nil
}

// Defaults for a built ad-hoc line that left its measure out.
const (
	adHocDefaultReps        = 8
	adHocDefaultHoldSeconds = 30 // a warm-up or cool-down hold
	adHocDefaultWorkSeconds = 45 // a timed main-work interval
)

// fillAdHocDefaults gives a built line the numbers it left out: 3 sets (1 for
// a warm-up or cool-down) of 8 reps, or of a hold for a timed exercise, at the
// user-built RPE.
func fillAdHocDefaults(line *roadmap.PrescribedExercise, main, timed bool) {
	if line.TargetSets == 0 {
		line.TargetSets = 1
		if main {
			line.TargetSets = 3
		}
	}
	if line.TargetReps == 0 && line.DurationSeconds == 0 {
		switch {
		case !timed:
			line.TargetReps = adHocDefaultReps
		case main:
			line.DurationSeconds = adHocDefaultWorkSeconds
		default:
			line.DurationSeconds = adHocDefaultHoldSeconds
		}
	}
	if main && line.TargetRPE == 0 {
		line.TargetRPE = adHocTargetRPE
	}
}

// lookUp reads the exercises from the catalog, in order.
func (h *CreateAdhocSessionHandler) lookUp(ctx context.Context, ids []string) ([]port.Exercise, error) {
	out := make([]port.Exercise, 0, len(ids))
	for _, id := range ids {
		ex, err := h.exercises.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get exercise %s: %w", id, err)
		}
		out = append(out, ex)
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
