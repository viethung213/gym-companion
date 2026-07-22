// Package services provides domain calculations for coaching.
package services

import "math"

// Calculate1RM calculates estimated One Rep Max using Epley's formula: 1RM = weight * (1 + reps / 30).
// Returns 0 if weight <= 0 or reps <= 0.
// Returns weight if reps == 1.
func Calculate1RM(weight float64, reps int) float64 {
	if weight <= 0 || reps <= 0 {
		return 0
	}
	if reps == 1 {
		return weight
	}

	return weight * (1.0 + float64(reps)/30.0)
}

// CalculateSuggestedWeight calculates suggested weight given 1RM and target reps: weight = 1RM / (1 + reps / 30).
// Returns 0 if oneRM <= 0 or targetReps <= 0.
// Returns oneRM if targetReps == 1.
func CalculateSuggestedWeight(oneRM float64, targetReps int) float64 {
	if oneRM <= 0 || targetReps <= 0 {
		return 0
	}
	if targetReps == 1 {
		return oneRM
	}

	return oneRM / (1.0 + float64(targetReps)/30.0)
}

// CalculateSuggestedReps calculates suggested reps given 1RM and target weight: reps = 30 * (1RM / weight - 1).
// Returns 0 if oneRM <= 0 or targetWeight <= 0.
// Returns 1 if targetWeight >= oneRM.
func CalculateSuggestedReps(oneRM float64, targetWeight float64) int {
	if oneRM <= 0 || targetWeight <= 0 {
		return 0
	}
	if targetWeight >= oneRM {
		return 1
	}

	reps := 30.0 * ((oneRM / targetWeight) - 1.0)
	return int(math.Round(reps))
}
