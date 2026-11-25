// Package guardrail is the deterministic hard gate a roadmap the coach plans or
// rewrites (initial plan, regeneration, adaptation) passes before it is saved,
// whatever the model wrote. It enforces:
//
//   - BR-AC-01: at most MaxSessionsPerWeek TRAINING sessions a week;
//   - BR-AC-02: a loaded main exercise at most 30% above the athlete's record;
//   - BR-AC-09: no exercise an active injury excludes, none above its RPE cap;
//   - BR-AC-10: scheduled main work inside its phase's RPE window;
//   - BR-AC-11: the DELOAD week below DeloadVolumeRatio of PEAK's volume;
//   - BR-AC-12: every exercise exists in the catalog;
//   - BR-AC-13: every line within the hard prescription limits.
//
// The engine keeps no state between checks. It reads the catalog and the
// athlete's records through the ports it is given.
package guardrail

import (
	"context"
	"strconv"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// Status of a review.
type Status string

const (
	StatusApproved Status = "APPROVED"

	StatusRejected Status = "REJECTED"
)

// Severity of an individual violation.
type Severity string

const (
	SeverityBlocker Severity = "BLOCKER"

	SeverityWarning Severity = "WARNING"
)

// Violation is one guard failure.
type Violation struct {
	Code        string
	Description string
	Path        string
	Severity    Severity
}

// ReviewResult is the unified shape returned by the guardrail check.
type ReviewResult struct {
	Status     Status
	Violations []Violation
}

// PRLookup returns a user's personal-record baseline weight per exercise id
// (port.WeightBaselines over the workout history). An exercise without history
// is absent; a nil map means none is known.
type PRLookup func(ctx context.Context, userID string) map[string]float64

// Engine applies all guardrail checks against a Roadmap.
type Engine struct {
	overload *service.OverloadValidator
	catalog  port.ExerciseCatalogReader
	prs      PRLookup
}

// NewEngine builds a guardrail engine.
func NewEngine(overload *service.OverloadValidator, catalog port.ExerciseCatalogReader, prs PRLookup) *Engine {
	if overload == nil {
		overload = service.NewOverloadValidator()
	}

	if prs == nil {
		prs = func(context.Context, string) map[string]float64 { return nil }
	}

	return &Engine{overload: overload, catalog: catalog, prs: prs}
}

// Check runs every rule against the given roadmap for a user with the given
// active injuries. Catalog existence (BR-AC-12) and the injury rules (BR-AC-09,
// the injury cap on BR-AC-10) only apply to sessions the caller can still
// change: PENDING sessions that have not started at from, the same set as
// Roadmap.UpcomingPendingSessions. Finished and past sessions are history. A
// zero from covers every pending session, as for a freshly drafted roadmap.
func (e *Engine) Check(ctx context.Context, r *roadmap.Roadmap, injuries []service.Injury, from time.Time) ReviewResult {
	if r == nil {
		return ReviewResult{
			Status: StatusRejected,

			Violations: []Violation{{Code: "INTERNAL_NIL", Description: "nil roadmap", Severity: SeverityBlocker}},
		}
	}

	loc := r.Location()
	inScope := func(s *roadmap.SessionPlan) bool {
		return s.Status() == roadmap.SessionPlanStatusPending && (from.IsZero() || s.StartsAt(loc).After(from))
	}

	traits, catalogViolations := e.checkExerciseCatalogExistence(ctx, r, inScope)
	verdicts := injuryVerdicts(traits, injuries)

	var vs []Violation

	vs = append(vs, e.checkWeeklyCap(r)...)

	vs = append(vs, checkPrescriptionLimits(r, inScope)...)

	vs = append(vs, e.checkWeightBand(ctx, r, inScope)...)

	vs = append(vs, checkInjuryExclusion(r, verdicts, inScope)...)

	vs = append(vs, checkPhaseRPEBand(r, verdicts, inScope)...)

	vs = append(vs, e.checkDeloadVolume(r)...)

	vs = append(vs, catalogViolations...)

	if len(vs) > 0 {
		return ReviewResult{Status: StatusRejected, Violations: vs}
	}

	return ReviewResult{Status: StatusApproved}
}

// checkExerciseCatalogExistence enforces BR-AC-12 and returns the traits of
// every exercise it found, keyed by exercise id.
func (e *Engine) checkExerciseCatalogExistence(
	ctx context.Context, r *roadmap.Roadmap, inScope func(*roadmap.SessionPlan) bool,
) (map[string]service.ExerciseTraits, []Violation) {
	var vs []Violation
	traits := make(map[string]service.ExerciseTraits)
	missing := make(map[string]string)

	for _, w := range r.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if !inScope(s) {
					continue // history cannot change; its exercises may since have left the catalog
				}
				for i, ex := range allExercises(s) {
					path := "sessions." + s.ID() + "[" + itoa(i) + "]"
					if ex.ExerciseID == "" {
						vs = append(vs, Violation{
							Code:        "BR-AC-12",
							Description: "prescribed exercise missing exercise_id",
							Path:        path,
							Severity:    SeverityBlocker,
						})
						continue
					}
					if ex.ExerciseName == "" {
						vs = append(vs, Violation{
							Code:        "BR-AC-12",
							Description: "exercise_id " + ex.ExerciseID + " has no catalog-resolved name",
							Path:        path,
							Severity:    SeverityBlocker,
						})
						continue
					}
					if e.catalog == nil {
						continue
					}
					if _, ok := traits[ex.ExerciseID]; !ok {
						if _, known := missing[ex.ExerciseID]; !known {
							e.lookup(ctx, ex.ExerciseID, traits, missing)
						}
					}
					if reason, bad := missing[ex.ExerciseID]; bad {
						vs = append(vs, Violation{
							Code:        "BR-AC-12",
							Description: "exercise_id " + ex.ExerciseID + " does not exist in exercise catalog: " + reason,
							Path:        path,
							Severity:    SeverityBlocker,
						})
					}
				}
			}
		}
	}

	return traits, vs
}

// lookup memoizes one catalog read: a 4-week plan repeats ~15 distinct ids.
func (e *Engine) lookup(ctx context.Context, id string, traits map[string]service.ExerciseTraits, missing map[string]string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	ex, err := e.catalog.GetByID(ctx, id)
	if err != nil {
		missing[id] = err.Error()
		return
	}
	traits[id] = ex.Traits()
}

// allExercises lists warm-ups, main work and cool-downs in order.
func allExercises(s *roadmap.SessionPlan) []roadmap.PrescribedExercise {
	presc := s.Info().Prescription
	all := append([]roadmap.PrescribedExercise{}, presc.WarmUps...)
	all = append(all, presc.MainExercises...)
	return append(all, presc.CoolDowns...)
}

// injuryVerdicts evaluates every known exercise against the injuries (§8.2).
func injuryVerdicts(traits map[string]service.ExerciseTraits, injuries []service.Injury) map[string]service.Verdict {
	verdicts := make(map[string]service.Verdict, len(traits))
	if len(injuries) == 0 {
		return verdicts
	}
	for id, t := range traits {
		verdicts[id] = service.EvaluateInjuries(&t, injuries)
	}
	return verdicts
}

// checkWeeklyCap enforces BR-AC-01.
func (e *Engine) checkWeeklyCap(r *roadmap.Roadmap) []Violation {
	var vs []Violation

	for _, w := range r.Weeks() {
		if w.TrainingSessions() > roadmap.MaxSessionsPerWeek {
			vs = append(vs, Violation{
				Code: "BR-AC-01",

				Description: "week exceeds max sessions",

				Path: "weeks[" + itoa(int(w.WeekNumber())) + "]",

				Severity: SeverityBlocker,
			})
		}
	}

	return vs
}

// checkPrescriptionLimits rejects numbers no sane prescription carries (sets,
// reps, duration, rest, RPE, load), whatever the model wrote.
func checkPrescriptionLimits(r *roadmap.Roadmap, inScope func(*roadmap.SessionPlan) bool) []Violation {
	var vs []Violation

	for _, w := range r.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if !inScope(s) {
					continue
				}
				if reason := roadmap.SessionDurationIssue(s.Info().EstimatedDurationMinutes); reason != "" {
					vs = append(vs, Violation{
						Code: "BR-AC-13", Description: reason,
						Path: "sessions." + s.ID(), Severity: SeverityBlocker,
					})
				}
				for i, ex := range allExercises(s) {
					if reason := ex.LimitIssue(); reason != "" {
						vs = append(vs, Violation{
							Code:        "BR-AC-13",
							Description: "exercise " + ex.ExerciseID + ": " + reason,
							Path:        "sessions." + s.ID() + "[" + itoa(i) + "]",
							Severity:    SeverityBlocker,
						})
					}
				}
			}
		}
	}

	return vs
}

// checkWeightBand enforces BR-AC-02 on sessions still in scope: a loaded main
// exercise with a personal record must stay at most 30% above it. History is
// left alone, since the record may have moved since it was planned.
func (e *Engine) checkWeightBand(ctx context.Context, r *roadmap.Roadmap, inScope func(*roadmap.SessionPlan) bool) []Violation {
	baselines := e.prs(ctx, r.UserID())
	if len(baselines) == 0 {
		return nil
	}

	var vs []Violation

	for _, w := range r.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if !inScope(s) {
					continue
				}
				presc := s.Info().Prescription

				for i, ex := range presc.MainExercises {
					baseline := baselines[ex.ExerciseID]

					// No record, or no load prescribed (bodyweight, timed work).
					if baseline <= 0 || ex.TargetWeight <= 0 {
						continue
					}

					if float64(ex.TargetWeight) > e.overload.Ceiling(baseline) {
						vs = append(vs, Violation{
							Code: "BR-AC-02",

							Description: "target weight more than 30% above the athlete's record",

							Path: "sessions." + s.ID() + ".main[" + itoa(i) + "]",

							Severity: SeverityBlocker,
						})
					}
				}
			}
		}
	}

	return vs
}

// checkInjuryExclusion enforces BR-AC-09 on every exercise of every session
// (warm-up, main, cool-down): an exercise the injury table excludes, or one
// prescribed above its injury RPE cap, is a blocker (docs/05 §8.2).
func checkInjuryExclusion(
	r *roadmap.Roadmap, verdicts map[string]service.Verdict, inScope func(*roadmap.SessionPlan) bool,
) []Violation {
	if len(verdicts) == 0 {
		return nil
	}

	var vs []Violation

	for _, w := range r.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if !inScope(s) {
					continue // completed, past or otherwise frozen sessions cannot be rewritten
				}
				for i, ex := range allExercises(s) {
					v, ok := verdicts[ex.ExerciseID]
					if !ok {
						continue
					}
					path := "sessions." + s.ID() + "[" + itoa(i) + "]"
					switch {
					case v.Excluded:
						vs = append(vs, Violation{
							Code:        "BR-AC-09",
							Description: "exercise " + ex.ExerciseID + " is excluded by an active injury: " + v.Reason(),
							Path:        path,
							Severity:    SeverityBlocker,
						})
					case v.MaxRPE > 0 && float64(ex.TargetRPE) > v.MaxRPE:
						vs = append(vs, Violation{
							Code: "BR-AC-09",
							Description: "exercise " + ex.ExerciseID + " target RPE " + ftoa(float64(ex.TargetRPE)) +
								" exceeds the injury cap " + ftoa(v.MaxRPE) + ": " + v.Reason(),
							Path:     path,
							Severity: SeverityBlocker,
						})
					}
				}
			}
		}
	}

	return vs
}

// checkPhaseRPEBand enforces BR-AC-10: main work must sit inside its phase's
// intensity window, so a PEAK-intensity session cannot hide inside a DELOAD
// week. Only sessions still in scope are checked, and an injury RPE cap lowers
// their window: safety wins over periodization.
//
// Warm-ups and cool-downs are excluded on purpose: they are prescribed at
// RPE 2-3 by design, so holding them to the phase window would reject every
// well-formed plan.
func checkPhaseRPEBand(
	r *roadmap.Roadmap, verdicts map[string]service.Verdict, inScope func(*roadmap.SessionPlan) bool,
) []Violation {
	var vs []Violation

	for _, w := range r.Weeks() {
		phaseBand, known := roadmap.PhaseRPEBand(w.Phase())
		if !known {
			continue // an unknown phase is ErrInvalidPhase's problem, not this rule's
		}

		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				// Periodization governs the scheduled program. An ad-hoc or solo
				// session is the athlete's own pick: holding it to the week's window
				// would make every later regeneration of the roadmap fail.
				if s.Info().Source != roadmap.SessionPlanSourceScheduled {
					continue
				}
				// A past, completed or skipped session cannot be rewritten. One
				// legally lowered by an injury cap would otherwise fail the window
				// once it leaves scope and block every later rewrite.
				if !inScope(s) {
					continue
				}
				// Recovery sessions follow their own ceiling, not the phase window.
				sessionBand := phaseBand
				if s.Kind() == roadmap.SessionKindRecovery {
					sessionBand = roadmap.RecoveryRPEBand()
				}
				for i, ex := range s.Info().Prescription.MainExercises {
					// A zero RPE means the model omitted the field rather than
					// prescribing an easy set; that is a schema defect the
					// validator owns, and rejecting it here would misreport it.
					if ex.TargetRPE == 0 {
						continue
					}

					band := sessionBand.Capped(float32(verdicts[ex.ExerciseID].MaxRPE))
					if !band.Contains(ex.TargetRPE) {
						vs = append(vs, Violation{
							Code: "BR-AC-10",

							Description: rpeWindowName(s, w.Phase()),

							Path: "sessions." + s.ID() + ".main[" + itoa(i) + "]",

							Severity: SeverityBlocker,
						})
					}
				}
			}
		}
	}

	return vs
}

func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// checkDeloadVolume enforces BR-AC-11: the DELOAD week must actually unload,
// carrying at most roadmap.DeloadVolumeRatio of the PEAK week's working sets.
//
// Dropping RPE while keeping volume intact passes BR-AC-10 and still leaves
// the athlete without a recovery week, which is what this rule catches.
func (e *Engine) checkDeloadVolume(r *roadmap.Roadmap) []Violation {
	peak := workingSets(r, roadmap.PhasePeak)

	// No PEAK week means no baseline to unload from; ValidateFullStructure
	// owns the "is this roadmap shaped correctly" question.
	if peak == 0 {
		return nil
	}

	deload := workingSets(r, roadmap.PhaseDeload)

	allowed := float64(peak) * roadmap.DeloadVolumeRatio
	if float64(deload) <= allowed {
		return nil
	}

	return []Violation{{
		Code: "BR-AC-11",

		Description: "DELOAD week carries " + itoa(deload) + " working sets, above the " +
			itoa(int(allowed)) + " allowed against PEAK's " + itoa(peak),

		Path: "weeks.DELOAD",

		Severity: SeverityBlocker,
	}}
}

// workingSets totals the main-exercise sets across every week in the phase.
func workingSets(r *roadmap.Roadmap, phase roadmap.Phase) int {
	total := 0

	for _, w := range r.Weeks() {
		if w.Phase() != phase {
			continue
		}

		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				// Only the scheduled program counts: recovery work is not training
				// volume, and an ad-hoc or solo session is the athlete's own pick, so
				// counting it could lock every later rewrite of the roadmap.
				if s.Kind() == roadmap.SessionKindRecovery || s.Info().Source != roadmap.SessionPlanSourceScheduled {
					continue
				}
				for _, ex := range s.Info().Prescription.MainExercises {
					total += int(ex.TargetSets)
				}
			}
		}
	}

	return total
}

func itoa(n int) string {
	// Small allocation-free int to string for path building.

	if n == 0 {
		return "0"
	}

	neg := false

	if n < 0 {
		neg = true

		n = -n
	}

	buf := [20]byte{}

	i := len(buf)

	for n > 0 {
		i--

		buf[i] = byte('0' + n%10)

		n /= 10
	}

	if neg {
		i--

		buf[i] = '-'
	}

	return string(buf[i:])
}

// rpeWindowName describes the RPE window a session's exercises must respect.
func rpeWindowName(s *roadmap.SessionPlan, phase roadmap.Phase) string {
	if s.Kind() == roadmap.SessionKindRecovery {
		return "target RPE above the recovery-session ceiling"
	}
	return "target RPE outside the " + string(phase) + " phase window"
}
