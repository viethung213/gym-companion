package service

import "math"

// SCRCalculator computes Session Completion Rate, ΔRPE and estimated 1RM.
// Formulas follow BR-AC-04 and standard Epley estimation.
//
// D7: ΔRPE uses scalar average_rpe from WorkoutSessionCompleted event
// (not per-set arrays). Formula: ΔRPE = avgActual - avgTarget.
type SCRCalculator struct{}

// NewSCRCalculator constructs the domain service.
func NewSCRCalculator() *SCRCalculator { return &SCRCalculator{} }

// SCR returns Session Completion Rate as a percentage (0..100).
// prescribedSets <= 0 returns 0 (avoid divide-by-zero); actual > prescribed caps at 100.
func (c *SCRCalculator) SCR(actualSets, prescribedSets int) float64 {
	if prescribedSets <= 0 {
		return 0
	}
	if actualSets < 0 {
		actualSets = 0
	}
	pct := float64(actualSets) / float64(prescribedSets) * 100.0
	if pct > 100.0 {
		return 100.0
	}
	return pct
}

// DeltaRPE returns (actual - target). Positive means the user found it harder
// than prescribed; negative means it felt too light.
func (c *SCRCalculator) DeltaRPE(actualAvgRPE, targetRPE float64) float64 {
	return actualAvgRPE - targetRPE
}

// EpleyOneRepMax returns the estimated one-rep max via Epley formula:
//   1RM = weight * (1 + reps/30)
// reps <= 0 or weight <= 0 returns 0.
func (c *SCRCalculator) EpleyOneRepMax(weight float64, reps int) float64 {
	if weight <= 0 || reps <= 0 {
		return 0
	}
	// 1 rep already IS a 1RM.
	if reps == 1 {
		return weight
	}
	return weight * (1.0 + float64(reps)/30.0)
}

// WeeklySCR aggregates SCR across multiple sessions:
// SCR_week = sum(actual) / sum(prescribed) * 100.
func (c *SCRCalculator) WeeklySCR(actualPerSession, prescribedPerSession []int) float64 {
	var a, p int
	for _, v := range actualPerSession {
		if v > 0 {
			a += v
		}
	}
	for _, v := range prescribedPerSession {
		if v > 0 {
			p += v
		}
	}
	return c.SCR(a, p)
}

// RoundToNearest rounds x to the nearest multiple of step. Useful for
// snapping generated weights to 2.5 kg or 5 lb increments.
func RoundToNearest(x, step float64) float64 {
	if step <= 0 {
		return x
	}
	return math.Round(x/step) * step
}
