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
			Status:     StatusRejected,
			Violations: []Violation{{Code: "INTERNAL_NIL", Description: "nil roadmap", Severity: SeverityBlocker}},
		}
	}
	var vs []Violation
	vs = append(vs, e.checkWeeklyCap(r)...)
	vs = append(vs, e.checkWeightBand(r)...)
	vs = append(vs, e.checkInjuryExclusion(r)...)
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
				Code:        "BR-AC-01",
				Description: "week exceeds max sessions",
				Path:        "weeks[" + itoa(int(w.WeekNumber())) + "]",
				Severity:    SeverityBlocker,
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
							Code:        "BR-AC-02",
							Description: "target weight outside ±30% of PR/baseline",
							Path:        "sessions." + s.ID() + ".main[" + itoa(i) + "]",
							Severity:    SeverityBlocker,
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
							Code:        "BR-AC-09",
							Description: "session targets injured muscle group: " + mg,
							Path:        "sessions." + s.ID(),
							Severity:    SeverityBlocker,
						})
					}
				}
			}
		}
	}
	return vs
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
