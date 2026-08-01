package adk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// Mirrored from the domain so violations become retryable issues, not AddDay errors.
const (
	maxSessionsPerWeek = roadmap.MaxSessionsPerWeek
	maxDaysPerWeek     = roadmap.DaysPerWeek
	scheduledDateISO   = "2006-01-02"
)

// planIssue describes one defect found in a generated plan.
type planIssue struct {
	WeekNumber int
	SessionIdx int
	Slot       string // "warm_ups" | "main_exercises" | "cool_downs"; empty for session/week issues
	ExerciseID string
	Reason     string
}

// String renders the issue as feedback for the model.
func (p planIssue) String() string {
	loc := fmt.Sprintf("week %d", p.WeekNumber)
	if p.SessionIdx >= 0 {
		loc += fmt.Sprintf(", session %d", p.SessionIdx+1)
	}
	if p.Slot != "" {
		loc += ", " + p.Slot
	}
	if p.ExerciseID != "" {
		return fmt.Sprintf("%s: exercise_id %q %s", loc, p.ExerciseID, p.Reason)
	}
	return loc + ": " + p.Reason
}

// validationOutcome is the result of validating one candidate plan.
type validationOutcome struct {
	Plan     *GeneratedPlan    // input plan in strict mode, filtered copy in salvage mode
	Names    map[string]string // exercise_id -> catalog display name
	Issues   []planIssue
	Degraded bool // salvage mode removed something
}

// planValidator checks exercise IDs against the catalog and plan shape against the domain.
type planValidator struct {
	catalog port.ExerciseCatalogReader

	// When > 0, the plan must hold exactly this many sessions: rewrite flows
	// match results back by position, so a miscount silently shifts them.
	wantSessions int
}

func newPlanValidator(catalog port.ExerciseCatalogReader) *planValidator {
	return &planValidator{catalog: catalog}
}

func (v *planValidator) expecting(sessions int) *planValidator {
	return &planValidator{catalog: v.catalog, wantSessions: sessions}
}

// validate reports plan defects; salvage mode drops them instead. Errors mean infrastructure.
func (v *planValidator) validate(
	ctx context.Context,
	plan *GeneratedPlan,
	dropInvalid bool,
) (validationOutcome, error) {
	if plan == nil {
		return validationOutcome{
			Issues: []planIssue{{SessionIdx: -1, Reason: "plan is empty"}},
		}, nil
	}

	if v.wantSessions > 0 {
		if got := planSessionCount(plan); got != v.wantSessions {
			return validationOutcome{
				Plan: plan,
				Issues: []planIssue{{SessionIdx: -1, Reason: fmt.Sprintf(
					"returned %d sessions but %d were given to revise; return exactly one per session, in the same order",
					got, v.wantSessions)}},
			}, nil
		}
	}

	names := make(map[string]string)
	lookupErr := make(map[string]bool) // exercise_id -> known-missing
	var issues []planIssue

	// Memoized: a 4-week plan spans ~96 slots but only ~15 distinct IDs.
	resolve := func(id string) (bool, error) {
		if _, ok := names[id]; ok {
			return true, nil
		}
		if lookupErr[id] {
			return false, nil
		}

		ex, err := v.catalog.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, port.ErrExerciseNotFound) {
				lookupErr[id] = true
				return false, nil
			}
			return false, fmt.Errorf("look up exercise %s: %w", id, err)
		}

		names[id] = ex.Name
		return true, nil
	}

	out := &GeneratedPlan{}
	degraded := false

	for _, wp := range plan.Weeks {
		weekIssues, keptSessions, weekDropped, err := v.validateWeek(ctx, wp, resolve, dropInvalid)
		if err != nil {
			return validationOutcome{}, err
		}
		issues = append(issues, weekIssues...)

		if !dropInvalid {
			continue
		}

		// Loss shapes: exercise dropped, session dropped, week dropped.
		if weekDropped || len(keptSessions) != len(wp.Sessions) {
			degraded = true
		}
		if len(keptSessions) == 0 {
			continue
		}

		kept := wp
		kept.Sessions = keptSessions
		out.Weeks = append(out.Weeks, kept)
	}

	if !dropInvalid {
		return validationOutcome{Plan: plan, Names: names, Issues: issues}, nil
	}

	if len(out.Weeks) != len(plan.Weeks) {
		degraded = true
	}

	return validationOutcome{
		Plan:     out,
		Names:    names,
		Issues:   issues,
		Degraded: degraded,
	}, nil
}

// validateWeek checks one week; kept and dropped apply only in salvage mode.
func (v *planValidator) validateWeek(
	ctx context.Context,
	wp WeekPlan,
	resolve func(string) (bool, error),
	dropInvalid bool,
) (issues []planIssue, kept []SessionPlan, dropped bool, err error) {
	distinctDates := make(map[string]struct{})

	for i := range wp.Sessions {
		sp := &wp.Sessions[i]
		sessionIssues, cleaned, sessionErr := v.validateSession(ctx, wp.WeekNumber, i, sp, resolve, dropInvalid)
		if sessionErr != nil {
			return nil, nil, false, sessionErr
		}
		issues = append(issues, sessionIssues...)

		if cleaned == nil {
			continue
		}

		if prescriptionSize(cleaned.Prescription) != prescriptionSize(sp.Prescription) {
			dropped = true
		}

		distinctDates[cleaned.ScheduledDate] = struct{}{}
		kept = append(kept, *cleaned)
	}

	// Count survivors so salvage does not flag a cap that dropping resolved.
	countFor := len(wp.Sessions)
	datesFor := countDistinctDates(wp.Sessions)
	if dropInvalid {
		countFor = len(kept)
		datesFor = len(distinctDates)
	}

	if countFor > maxSessionsPerWeek {
		issues = append(issues, planIssue{
			WeekNumber: wp.WeekNumber,
			SessionIdx: -1,
			Reason: fmt.Sprintf("has %d sessions, exceeding the limit of %d per week",
				countFor, maxSessionsPerWeek),
		})
	}
	if datesFor > maxDaysPerWeek {
		issues = append(issues, planIssue{
			WeekNumber: wp.WeekNumber,
			SessionIdx: -1,
			Reason: fmt.Sprintf("spans %d distinct dates, exceeding the limit of %d per week",
				datesFor, maxDaysPerWeek),
		})
	}

	return issues, kept, dropped, nil
}

// prescriptionSize counts the exercises across all three slots.
func prescriptionSize(p WorkoutPrescription) int {
	return len(p.WarmUps) + len(p.MainExercises) + len(p.CoolDowns)
}

// validateSession returns a cleaned copy in salvage mode, nil if unsalvageable.
func (v *planValidator) validateSession(
	_ context.Context,
	weekNumber, idx int,
	sp *SessionPlan,
	resolve func(string) (bool, error),
	dropInvalid bool,
) (issues []planIssue, kept *SessionPlan, err error) {
	if _, parseErr := time.Parse(scheduledDateISO, sp.ScheduledDate); parseErr != nil {
		issues = append(issues, planIssue{
			WeekNumber: weekNumber,
			SessionIdx: idx,
			Reason: fmt.Sprintf("scheduled_date %q is not a valid YYYY-MM-DD date",
				sp.ScheduledDate),
		})
		// Unsalvageable: the domain requires a non-zero ScheduledDate.
		return issues, nil, nil
	}

	slots := []struct {
		name string
		exs  []PrescribedExercise
	}{
		{"warm_ups", sp.Prescription.WarmUps},
		{"main_exercises", sp.Prescription.MainExercises},
		{"cool_downs", sp.Prescription.CoolDowns},
	}

	cleanedSlots := make([][]PrescribedExercise, len(slots))
	for i, slot := range slots {
		for _, ex := range slot.exs {
			ok, resolveErr := resolve(ex.ExerciseID)
			if resolveErr != nil {
				return nil, nil, resolveErr
			}
			if !ok {
				issues = append(issues, planIssue{
					WeekNumber: weekNumber,
					SessionIdx: idx,
					Slot:       slot.name,
					ExerciseID: ex.ExerciseID,
					Reason:     "is not in the exercise catalog",
				})
				continue
			}
			cleanedSlots[i] = append(cleanedSlots[i], ex)
		}
	}

	if len(sp.Prescription.MainExercises) == 0 {
		issues = append(issues, planIssue{
			WeekNumber: weekNumber,
			SessionIdx: idx,
			Slot:       "main_exercises",
			Reason:     "is empty; a session must prescribe at least one main exercise",
		})
	}

	if !dropInvalid {
		return issues, nil, nil
	}

	if len(cleanedSlots[1]) == 0 {
		return issues, nil, nil
	}

	cleaned := *sp
	cleaned.Prescription = WorkoutPrescription{
		WarmUps:       cleanedSlots[0],
		MainExercises: cleanedSlots[1],
		CoolDowns:     cleanedSlots[2],
	}
	return issues, &cleaned, nil
}

// countDistinctDates counts unique dates, malformed included: the cap is about days asked for.
func countDistinctDates(sessions []SessionPlan) int {
	seen := make(map[string]struct{}, len(sessions))
	for i := range sessions {
		seen[sessions[i].ScheduledDate] = struct{}{}
	}
	return len(seen)
}

// planSessionCount reports the total number of sessions across all weeks.
func planSessionCount(plan *GeneratedPlan) int {
	if plan == nil {
		return 0
	}
	total := 0
	for _, wp := range plan.Weeks {
		total += len(wp.Sessions)
	}
	return total
}
