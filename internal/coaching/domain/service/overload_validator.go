// Package service contains domain services for the Coaching context:
// OverloadValidator (BR-AC-02 load ceiling), SCRCalculator (BR-AC-04 formulas),
// and AdaptiveCoachEngine (BR-AC-04..08 signal detection).
package service

// OverloadBand is how far above a baseline a prescribed load may go.
// BR-AC-02: at most 30% above the athlete's record ⇒ 0.30.
const OverloadBand = 0.30

// OverloadValidator bounds prescribed loads against a baseline, the athlete's
// heaviest recorded weight for the exercise.
type OverloadValidator struct{}

// NewOverloadValidator constructs the domain service.
func NewOverloadValidator() *OverloadValidator {
	return &OverloadValidator{}
}

// Ceiling is the heaviest load BR-AC-02 allows against a baseline: the
// baseline plus OverloadBand. There is deliberately no floor: lighter work
// (deload, high reps, recovery) is never unsafe, and a record set for one rep
// says nothing against fifteen reps at half the load.
func (v *OverloadValidator) Ceiling(baseline float64) float64 {
	return baseline * (1.0 + OverloadBand)
}
