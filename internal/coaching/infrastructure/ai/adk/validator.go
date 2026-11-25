package adk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// Mirrored from the domain so violations become retryable issues, not AddDay errors.
const (
	maxSessionsPerWeek = roadmap.MaxSessionsPerWeek
	maxDaysPerWeek     = roadmap.DaysPerWeek
	maxSlotsPerWeek    = roadmap.MaxSlotsPerWeek
	scheduledDateISO   = "2006-01-02"
)

// planIssue describes one defect found in a generated plan.
type planIssue struct {
	WeekNumber int
	SessionIdx int
	Slot       string // "warm_ups" | "main_exercises" | "cool_downs"; empty for session/week issues
	ExerciseID string
	Reason     string
	// Fitted marks a defect the validator already fixed on the plan (fitRPE):
	// it is logged, and costs a retry only when too many lines needed it.
	Fitted bool
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
	Cautions map[string]string // exercise_id -> injury limit kept despite an injury
	// Primaries maps exercise_id -> primary MuscleGroup code for every resolved exercise.
	Primaries map[string]string
	Issues    []planIssue
	Fitted    []planIssue // fixed on the plan by the validator, see splitFitted
	Degraded  bool        // salvage mode removed something
}

// planValidator checks exercise IDs against the catalog and plan shape against the domain.
type planValidator struct {
	catalog port.ExerciseCatalogReader

	// When > 0, the plan must hold exactly this many sessions: rewrite flows
	// match results back by position, so a miscount silently shifts them.
	wantSessions int

	// athlete, when set, holds the equipment and injuries every prescribed
	// exercise must respect (docs/05 §8.2, §8.3).
	athlete *service.Athlete

	// schedule, when set, holds the initial roadmap's selected slots: every
	// open slot becomes one session, and no session may sit in the past or
	// reuse a slot. Rewrite flows never set it.
	schedule *slotSchedule

	// revise, set for rewrite flows, holds the sessions being revised: results
	// take their date, slot and kind by position.
	revise []SessionToRevise

	// loads, when set, bounds every prescribed weight to the athlete.
	loads *loadLimits
}

// loadLimits are the athlete-specific bounds on a prescribed weight.
type loadLimits struct {
	// baselines maps exercise_id to the athlete's personal-record weight: a
	// prescription may be at most 30% above it (BR-AC-02).
	baselines map[string]float64
	// bodyWeightKg caps the load of an exercise with no record yet; 0 = unknown.
	bodyWeightKg float64
}

func newPlanValidator(catalog port.ExerciseCatalogReader) *planValidator {
	return &planValidator{catalog: catalog}
}

func (v *planValidator) expecting(sessions int) *planValidator {
	cp := *v
	cp.wantSessions = sessions
	return &cp
}

func (v *planValidator) withAthlete(athlete *service.Athlete) *planValidator {
	cp := *v
	cp.athlete = athlete
	return &cp
}

func (v *planValidator) withSchedule(schedule *slotSchedule) *planValidator {
	cp := *v
	cp.schedule = schedule
	return &cp
}

func (v *planValidator) withRevise(revise []SessionToRevise) *planValidator {
	cp := *v
	cp.revise = revise
	return &cp
}

func (v *planValidator) withLoads(loads *loadLimits) *planValidator {
	cp := *v
	cp.loads = loads
	return &cp
}

// pinKinds sets every session's kind and phase (and, for rewrites, its date
// and slot) from what the backend scheduled: the model never decides them.
// An initial roadmap's weeks take their phase and RPE window from their
// number, so the model cannot skip the PEAK or DELOAD week.
func (v *planValidator) pinKinds(plan *GeneratedPlan) {
	if v.revise != nil {
		pinRewriteSlots(plan, v.revise)
	} else {
		for w := range plan.Weeks {
			wp := &plan.Weeks[w]
			if v.schedule != nil {
				pinWeekPhase(wp)
			}
			for i := range wp.Sessions {
				sp := &wp.Sessions[i]
				sp.SessionKind = kindTraining
				if v.schedule != nil {
					sp.SessionKind = v.schedule.kindOf(sp.ScheduledDate, sp.SlotTime)
					sp.phase = roadmap.Phase(wp.Phase)
				}
			}
		}
	}
	capRecoveryRPE(plan)
}

// pinWeekPhase sets a week's phase and RPE window from its number; a number
// outside the block is left for weekNumberIssues to report.
func pinWeekPhase(wp *WeekPlan) {
	phase := roadmap.PhaseForWeek(wp.WeekNumber)
	band, ok := roadmap.PhaseRPEBand(phase)
	if !ok {
		return
	}
	wp.Phase = string(phase)
	wp.TargetRPEMin = float64(band.Min)
	wp.TargetRPEMax = float64(band.Max)
}

// weekNumberIssues reports an initial roadmap whose weeks are not numbered
// 1..4 exactly once. In salvage mode it also renumbers the plan: the first
// week claiming a number keeps it, the rest are dropped (their sessions sat
// in another week's dates and validateSlot would drop them anyway), and a
// missing week is added empty so the roadmap still has all four.
func weekNumberIssues(plan *GeneratedPlan, salvage bool) []planIssue {
	var issues []planIssue
	byNumber := make(map[int]WeekPlan, roadmap.WeeksPerRoadmap)
	for _, wp := range plan.Weeks {
		if roadmap.PhaseForWeek(wp.WeekNumber) == "" {
			issues = append(issues, planIssue{WeekNumber: wp.WeekNumber, SessionIdx: -1,
				Reason: fmt.Sprintf("week_number must be 1 to %d", roadmap.WeeksPerRoadmap)})
			continue
		}
		if _, dup := byNumber[wp.WeekNumber]; dup {
			issues = append(issues, planIssue{WeekNumber: wp.WeekNumber, SessionIdx: -1,
				Reason: "appears twice; list each week once"})
			continue
		}
		byNumber[wp.WeekNumber] = wp
	}
	for n := 1; n <= roadmap.WeeksPerRoadmap; n++ {
		if _, ok := byNumber[n]; !ok {
			issues = append(issues, planIssue{WeekNumber: n, SessionIdx: -1,
				Reason: "is missing; the roadmap always has weeks 1 to 4"})
		}
	}
	if salvage && len(issues) > 0 {
		weeks := make([]WeekPlan, 0, roadmap.WeeksPerRoadmap)
		for n := 1; n <= roadmap.WeeksPerRoadmap; n++ {
			wp, ok := byNumber[n]
			if !ok {
				wp = WeekPlan{WeekNumber: n}
			}
			weeks = append(weeks, wp)
		}
		plan.Weeks = weeks
	}
	return issues
}

// exerciseResolver returns the catalog exercise, or nil when the id is unknown.
type exerciseResolver func(id string) (*port.Exercise, error)

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
	var issues []planIssue
	degraded := false
	if v.schedule != nil {
		issues = weekNumberIssues(plan, dropInvalid)
		degraded = dropInvalid && len(issues) > 0
	}
	v.pinKinds(plan)

	names := make(map[string]string)
	exercises := make(map[string]*port.Exercise)
	lookupErr := make(map[string]bool) // exercise_id -> known-missing
	cautions := make(map[string]string)
	primaries := make(map[string]string)

	// Memoized: a 4-week plan spans ~96 slots but only ~15 distinct IDs.
	resolve := func(id string) (*port.Exercise, error) {
		if ex, ok := exercises[id]; ok {
			return ex, nil
		}
		if lookupErr[id] {
			return nil, nil
		}

		ex, err := v.catalog.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, port.ErrExerciseNotFound) {
				lookupErr[id] = true
				return nil, nil
			}
			return nil, fmt.Errorf("look up exercise %s: %w", id, err)
		}

		names[id] = ex.Name
		primaries[id] = string(ex.PrimaryMuscle)
		exercises[id] = &ex
		return &ex, nil
	}

	out := &GeneratedPlan{}
	usedSlots := make(map[string]struct{}) // "date slot-start" already scheduled

	for _, wp := range plan.Weeks {
		weekIssues, keptSessions, weekDropped, err := v.validateWeek(
			ctx, wp, resolve, cautions, usedSlots, dropInvalid)
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
		// Keep a week even when nothing survived: a week may legitimately hold no
		// session (no open slot), and the roadmap always has all four weeks.
		kept := wp
		kept.Sessions = keptSessions
		out.Weeks = append(out.Weeks, kept)
	}

	// BR-AC-11 (DELOAD volume) is not checked here: the domain cuts the DELOAD
	// sets to fit before the guardrail, so a heavy DELOAD week never costs a
	// retry (roadmap.FitDeloadVolume).
	issues, fitted := splitFitted(plan, issues, dropInvalid)
	if !dropInvalid {
		return validationOutcome{
			Plan: plan, Names: names, Cautions: cautions, Primaries: primaries,
			Issues: issues, Fitted: fitted,
		}, nil
	}

	if len(out.Weeks) != len(plan.Weeks) {
		degraded = true
	}

	return validationOutcome{
		Plan:      out,
		Names:     names,
		Cautions:  cautions,
		Primaries: primaries,
		Issues:    issues,
		Fitted:    fitted,
		Degraded:  degraded,
	}, nil
}

// validateWeek checks one week; kept and dropped apply only in salvage mode.
func (v *planValidator) validateWeek(
	ctx context.Context,
	wp WeekPlan,
	resolve exerciseResolver,
	cautions map[string]string,
	usedSlots map[string]struct{},
	dropInvalid bool,
) (issues []planIssue, kept []SessionPlan, dropped bool, err error) {
	distinctDates := make(map[string]struct{})
	onOpenSlot := 0 // sessions neither past nor reusing a slot

	for i := range wp.Sessions {
		sp := &wp.Sessions[i]
		sessionIssues, cleaned, sessionErr := v.validateSession(ctx, wp.WeekNumber, i, sp, resolve, cautions, dropInvalid)
		if sessionErr != nil {
			return nil, nil, false, sessionErr
		}
		issues = append(issues, sessionIssues...)

		slotIssues, slotKey, slotOK := v.validateSlot(wp.WeekNumber, i, sp, usedSlots)
		issues = append(issues, slotIssues...)
		if slotOK && (!dropInvalid || cleaned != nil) {
			usedSlots[slotKey] = struct{}{}
			onOpenSlot++
		}

		if cleaned == nil {
			continue
		}
		if !slotOK {
			// Salvage: a past session cannot be trained and a reused slot
			// double-books the athlete, so neither ships.
			dropped = true
			continue
		}

		if prescriptionSize(cleaned.Prescription) != prescriptionSize(sp.Prescription) {
			dropped = true
		}

		distinctDates[cleaned.ScheduledDate] = struct{}{}
		kept = append(kept, *cleaned)
	}

	// The weekly caps hold for weeks the model lays out. A rewrite lists every
	// revised session under one entry, possibly spanning several weeks, and
	// the backend pins each one to its existing date and slot: the roadmap's
	// weeks already respect the caps, so counting that entry as one week would
	// reject every rewrite of more than six sessions.
	if v.revise == nil {
		// Count survivors so salvage does not flag a cap that dropping resolved.
		counted := wp.Sessions
		datesFor := countDistinctDates(wp.Sessions)
		if dropInvalid {
			counted = kept
			datesFor = len(distinctDates)
		}
		issues = append(issues, weekCapIssues(wp.WeekNumber, counted, datesFor)...)
	}

	if shortfall := v.weekShortfall(wp.WeekNumber, onOpenSlot); shortfall != nil {
		issues = append(issues, *shortfall)
		if dropInvalid {
			// Out of attempts: a thinner week still beats no roadmap, so the
			// shortfall travels as a caution instead of failing the plan.
			slog.WarnContext(ctx, "Initial roadmap week does not cover every selected slot",
				slog.Int("week", wp.WeekNumber),
				slog.Int("sessions", onOpenSlot),
				slog.Int("expected", v.schedule.expectedSessions(wp.WeekNumber)))
		}
	}

	return issues, kept, dropped, nil
}

// validateSlot checks a session against the initial roadmap's selected slots.
// ok is false when the session must not ship: unparseable date, already
// started at plan creation, or a slot an earlier session took. key identifies
// the slot; the caller records it once the session is kept.
func (v *planValidator) validateSlot(
	weekNumber, idx int,
	sp *SessionPlan,
	usedSlots map[string]struct{},
) (issues []planIssue, key string, ok bool) {
	if v.schedule == nil {
		return nil, "", true
	}
	date, err := time.Parse(scheduledDateISO, sp.ScheduledDate)
	if err != nil {
		return nil, "", false // validateSession already reported it
	}

	loc := planIssue{WeekNumber: weekNumber, SessionIdx: idx}
	// The roadmap's weeks share the schedule's anchor, so a session listed under
	// the wrong week would sit outside its WeekPlan's date range.
	if start := v.schedule.weekStart(weekNumber); date.Before(start) || !date.Before(start.AddDate(0, 0, 7)) {
		loc.Reason = fmt.Sprintf("scheduled_date %s is not in week %d (%s to %s); list it under the week whose week_slots hold it",
			sp.ScheduledDate, weekNumber, start.Format(scheduledDateISO), start.AddDate(0, 0, 6).Format(scheduledDateISO))
		return append(issues, loc), "", false
	}
	weekday := date.Weekday()
	if !v.schedule.matchesSlot(weekday, sp.SlotTime) {
		loc.Reason = fmt.Sprintf("slot_time %q on %s matches no selected slot for %s (selected: %v)",
			sp.SlotTime, sp.ScheduledDate, weekdayToCode(weekday), v.schedule.daySlots(weekday))
		issues = append(issues, loc)
	}

	if v.schedule.isPast(date, sp.SlotTime) {
		loc.Reason = fmt.Sprintf("slot %s %s starts before the plan was created (%s); "+
			"schedule only slots listed in week_slots",
			sp.ScheduledDate, sp.SlotTime, v.schedule.start.Format(time.RFC3339))
		return append(issues, loc), "", false
	}

	key = sp.ScheduledDate + " " + slotStartClock(sp.SlotTime)
	if _, taken := usedSlots[key]; taken {
		loc.Reason = fmt.Sprintf("slot %s %s is already used by another session; "+
			"each selected slot holds exactly one session", sp.ScheduledDate, sp.SlotTime)
		return append(issues, loc), "", false
	}

	return issues, key, true
}

// weekShortfall reports a week holding fewer sessions than it has open slots.
func (v *planValidator) weekShortfall(weekNumber, sessions int) *planIssue {
	if v.schedule == nil {
		return nil
	}
	want := v.schedule.expectedSessions(weekNumber)
	if sessions >= want {
		return nil
	}
	return &planIssue{
		WeekNumber: weekNumber,
		SessionIdx: -1,
		Reason: fmt.Sprintf("has %d sessions on open slots but expects %d: "+
			"schedule one session per slot in week_slots, with the session_kind listed there",
			sessions, want),
	}
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
	resolve exerciseResolver,
	cautions map[string]string,
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
		for j := range slot.exs {
			catalogEx, resolveErr := resolve(slot.exs[j].ExerciseID)
			if resolveErr != nil {
				return nil, nil, resolveErr
			}
			loc := planIssue{WeekNumber: weekNumber, SessionIdx: idx, Slot: slot.name, ExerciseID: slot.exs[j].ExerciseID}
			if catalogEx == nil {
				loc.Reason = "is not in the exercise catalog"
				issues = append(issues, loc)
				continue
			}
			// On the plan itself, not a copy: a plan that passes as it is ships as
			// it is, and must not carry reps on a timed exercise.
			dropStrayMeasureFields(&slot.exs[j], catalogEx.MeasurementType)
			ex := slot.exs[j]
			if sp.SessionKind == kindRecovery {
				if !recoveryCategory(catalogEx.Category) {
					loc.Reason = fmt.Sprintf("is a %s exercise; a RECOVERY session takes every exercise from recovery_exercises",
						catalogEx.Category)
					issues = append(issues, loc)
					continue
				}
			}

			var injuryCap float64
			if v.athlete != nil {
				traits := catalogEx.Traits()
				if !service.EquipmentAvailable(traits.Equipment, v.athlete.Equipment) {
					loc.Reason = "needs equipment " + string(traits.Equipment) +
						" the athlete does not have; pick an exercise using profile.available_equipment"
					issues = append(issues, loc)
					continue
				}
				verdict := service.EvaluateInjuries(&traits, v.athlete.Injuries)
				if verdict.Excluded {
					loc.Reason = "conflicts with an active injury (" + verdict.Reason() +
						"); replace it with an eligible exercise or drop it"
					issues = append(issues, loc)
					continue
				}
				if verdict.MaxRPE > 0 {
					injuryCap = verdict.MaxRPE
					cautions[ex.ExerciseID] = injuryLimitText(&verdict)
				}
			}

			// Strict mode fits the plan itself, like the measure fields, so a
			// plan whose only defect is RPE ships as fitted; salvage fits its copy.
			target := &ex
			if !dropInvalid {
				target = &slot.exs[j]
			}
			if fix := fitRPE(target, slot.name == "main_exercises", sp, injuryCap); fix != "" {
				fittedLoc := loc
				fittedLoc.Reason, fittedLoc.Fitted = fix, true
				issues = append(issues, fittedLoc)
				ex = *target
			}

			// Numbers last: salvage keeps the line, clamped to every limit.
			for _, reason := range v.clampLine(&ex, slot.name == "main_exercises", catalogEx.MeasurementType, sp, injuryCap) {
				loc.Reason = reason
				issues = append(issues, loc)
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

	// estimated_duration_minutes is not checked: the stored length is computed
	// from the prescription (roadmap.WorkoutPrescription.EstimatedMinutes).

	if !dropInvalid {
		return issues, nil, nil
	}

	if len(cleanedSlots[1]) == 0 {
		// Unsalvageable: every main exercise was invalid.
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

// countTraining counts the TRAINING sessions; an unset kind is TRAINING.
func countTraining(sessions []SessionPlan) int {
	n := 0
	for i := range sessions {
		if sessions[i].SessionKind != kindRecovery {
			n++
		}
	}
	return n
}

// recoveryCategory reports whether an exercise fits an active-recovery session.
func recoveryCategory(c taxonomy.Category) bool {
	return c == taxonomy.CategoryStretching || c == taxonomy.CategoryCardio
}

// capRecoveryRPE lowers every recovery exercise to the recovery ceiling: a
// deterministic fix, not worth a regeneration.
func capRecoveryRPE(plan *GeneratedPlan) {
	ceiling := float64(roadmap.RecoveryMaxRPE)
	for w := range plan.Weeks {
		for i := range plan.Weeks[w].Sessions {
			sp := &plan.Weeks[w].Sessions[i]
			if sp.SessionKind != kindRecovery {
				continue
			}
			for _, group := range [][]PrescribedExercise{sp.Prescription.WarmUps, sp.Prescription.MainExercises, sp.Prescription.CoolDowns} {
				for j := range group {
					group[j].TargetRPE = min(group[j].TargetRPE, ceiling)
				}
			}
		}
	}
}

// weekCapIssues reports the weekly caps a laid-out week breaks: TRAINING
// sessions (BR-AC-01), all sessions, and distinct dates.
func weekCapIssues(weekNumber int, sessions []SessionPlan, dates int) []planIssue {
	var issues []planIssue
	week := func(reason string) {
		issues = append(issues, planIssue{WeekNumber: weekNumber, SessionIdx: -1, Reason: reason})
	}
	if training := countTraining(sessions); training > maxSessionsPerWeek {
		week(fmt.Sprintf("has %d training sessions, exceeding the limit of %d per week", training, maxSessionsPerWeek))
	}
	if len(sessions) > maxSlotsPerWeek {
		week(fmt.Sprintf("has %d sessions, exceeding the limit of %d per week", len(sessions), maxSlotsPerWeek))
	}
	if dates > maxDaysPerWeek {
		week(fmt.Sprintf("spans %d distinct dates, exceeding the limit of %d per week", dates, maxDaysPerWeek))
	}
	return issues
}
