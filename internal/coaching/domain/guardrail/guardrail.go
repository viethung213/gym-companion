// Package guardrail is the deterministic Go guardrail layer applied AFTER
// (and optionally alongside) the LLM CoachAgent. Enforces BR-AC-01 (weekly
// cap), BR-AC-02 (±30% weight band), BR-AC-09 (injury exclusion / protection).
//
// D6: The engine is designed stateless + pure so it can also be exposed as an
// MCP tool for the Generator/Evaluator to consult during generation.
package guardrail

import (
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

// deloadVolumeRatio is the share of PEAK working sets a DELOAD week may keep.
const deloadVolumeRatio = 0.7

// rpeBand is the inclusive intensity window allowed for a phase's main work.
type rpeBand struct {
	min float32
	max float32
}

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

// PRLookup returns the personal-record baseline weight for a given exercise
// (typically supplied by ExerciseCatalogReader + WorkoutSessionReader in the
// concrete adapter). Return baseline<=0 when no history exists.
type PRLookup func(exerciseID string) float64

// Engine applies all guardrail checks against a Roadmap.
type Engine struct {
	overload *service.OverloadValidator
	prs      PRLookup
	injuries []port.InjuryStatus
}

// NewEngine builds a guardrail engine.
func NewEngine(overload *service.OverloadValidator, prs PRLookup, injuries []port.InjuryStatus) *Engine {
	if overload == nil {
		overload = service.NewOverloadValidator()
	}

	if prs == nil {
		prs = func(string) float64 { return 0 }
	}

	return &Engine{overload: overload, prs: prs, injuries: injuries}
}

// Check runs every rule against the given roadmap.
func (e *Engine) Check(r *roadmap.Roadmap) ReviewResult {
	if r == nil {
		return ReviewResult{
			Status: StatusRejected,

			Violations: []Violation{{Code: "INTERNAL_NIL", Description: "nil roadmap", Severity: SeverityBlocker}},
		}
	}

	var vs []Violation

	vs = append(vs, e.checkWeeklyCap(r)...)

	vs = append(vs, e.checkWeightBand(r)...)

	vs = append(vs, e.checkInjuryExclusion(r)...)

	vs = append(vs, e.checkPhaseRPEBand(r)...)

	vs = append(vs, e.checkDeloadVolume(r)...)

	if len(vs) > 0 {
		return ReviewResult{Status: StatusRejected, Violations: vs}
	}

	return ReviewResult{Status: StatusApproved}
}

// checkWeeklyCap enforces BR-AC-01.
func (e *Engine) checkWeeklyCap(r *roadmap.Roadmap) []Violation {
	var vs []Violation

	for _, w := range r.Weeks() {
		if w.TotalSessions() > roadmap.MaxSessionsPerWeek {
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

// checkWeightBand enforces BR-AC-02.
func (e *Engine) checkWeightBand(r *roadmap.Roadmap) []Violation {
	var vs []Violation

	for _, w := range r.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				presc := s.Info().Prescription

				for i, ex := range presc.MainExercises {
					baseline := e.prs(ex.ExerciseID)

					if baseline <= 0 {
						continue
					}

					if !e.overload.Within(float64(ex.TargetWeight), baseline) {
						vs = append(vs, Violation{
							Code: "BR-AC-02",

							Description: "target weight outside ±30% of PR/baseline",

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

// checkInjuryExclusion enforces BR-AC-09 (active injuries: exclude affected muscle groups).
func (e *Engine) checkInjuryExclusion(r *roadmap.Roadmap) []Violation {
	if len(e.injuries) == 0 {
		return nil
	}

	// Only consider injuries that are NOT yet recovered.

	active := map[string]struct{}{}

	for _, inj := range e.injuries {
		if inj.RecoveredAt == nil {
			active[inj.MuscleGroup] = struct{}{}
		}
	}

	if len(active) == 0 {
		return nil
	}

	var vs []Violation

	for _, w := range r.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				for _, mg := range s.Info().TargetMuscleGroups {
					if _, hit := active[mg]; hit {
						vs = append(vs, Violation{
							Code: "BR-AC-09",

							Description: "session targets injured muscle group: " + mg,

							Path: "sessions." + s.ID(),

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
// week.
//
// Warm-ups and cool-downs are excluded on purpose: they are prescribed at
// RPE 2-3 by design, so holding them to the phase window would reject every
// well-formed plan.
func (e *Engine) checkPhaseRPEBand(r *roadmap.Roadmap) []Violation {
	var vs []Violation

	for _, w := range r.Weeks() {
		band, known := rpeBandFor(w.Phase())
		if !known {
			continue // an unknown phase is ErrInvalidPhase's problem, not this rule's
		}

		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				for i, ex := range s.Info().Prescription.MainExercises {
					// A zero RPE means the model omitted the field rather than
					// prescribing an easy set; that is a schema defect the
					// validator owns, and rejecting it here would misreport it.
					if ex.TargetRPE == 0 {
						continue
					}

					if ex.TargetRPE < band.min || ex.TargetRPE > band.max {
						vs = append(vs, Violation{
							Code: "BR-AC-10",

							Description: "target RPE outside the " + string(w.Phase()) + " phase window",

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

// checkDeloadVolume enforces BR-AC-11: the DELOAD week must actually unload,
// carrying at most deloadVolumeRatio of the PEAK week's working sets.
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

	allowed := float64(peak) * deloadVolumeRatio
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
				for _, ex := range s.Info().Prescription.MainExercises {
					total += int(ex.TargetSets)
				}
			}
		}
	}

	return total
}

// rpeBandFor returns the inclusive RPE window for a phase, mirroring the
// phase table the generator prompt instructs the model to follow.
func rpeBandFor(p roadmap.Phase) (rpeBand, bool) {
	switch p {
	case roadmap.PhaseAccumulation:
		return rpeBand{min: 6.0, max: 7.0}, true
	case roadmap.PhaseOverload:
		return rpeBand{min: 7.0, max: 8.0}, true
	case roadmap.PhasePeak:
		return rpeBand{min: 8.0, max: 9.0}, true
	case roadmap.PhaseDeload:
		return rpeBand{min: 5.0, max: 6.0}, true
	default:
		return rpeBand{}, false
	}
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
