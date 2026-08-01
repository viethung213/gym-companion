// Package service contains domain services for the Coaching context:
// OverloadValidator (BR-AC-02 ±30% cap), SCRCalculator (BR-AC-04 formulas),
// and AdaptiveCoachEngine (BR-AC-04..08 signal detection).
package service

import "errors"

// ErrNoBaseline is returned when there is no history / PR baseline to cap against.
// Callers may treat this as "accept the suggested weight as-is" (new user, no PR yet).
var ErrNoBaseline = errors.New("no baseline weight available")

// OverloadBand is the maximum permitted delta as a fraction of baseline.
// BR-AC-02: ±30% ⇒ 0.30.
const OverloadBand = 0.30

// OverloadValidator caps suggested weights within ±30% of a baseline
// (typically the user's PR or last suggested weight for the exercise).
type OverloadValidator struct{}

// NewOverloadValidator constructs the domain service.
func NewOverloadValidator() *OverloadValidator {
	return &OverloadValidator{}
}

// Cap returns the weight bounded to [baseline*(1-band), baseline*(1+band)].
// If baseline <= 0, returns (suggested, ErrNoBaseline) so caller can decide.
func (v *OverloadValidator) Cap(suggested, baseline float64) (float64, error) {
	if baseline <= 0 {
		return suggested, ErrNoBaseline
	}
	upper := baseline * (1.0 + OverloadBand)
	lower := baseline * (1.0 - OverloadBand)
	switch {
	case suggested > upper:
		return upper, nil
	case suggested < lower:
		return lower, nil
	default:
		return suggested, nil
	}
}

// Within reports whether the suggested weight is inside the ±30% band.
// If baseline <= 0, returns true (no way to check).
func (v *OverloadValidator) Within(suggested, baseline float64) bool {
	if baseline <= 0 {
		return true
	}
	lo, hi := baseline*(1.0-OverloadBand), baseline*(1.0+OverloadBand)
	return suggested >= lo && suggested <= hi
}
