package domain

import (
	"fmt"
	"time"
)

type WorkoutPrescription struct {
	Sets        int
	Reps        int
	Weight      float64
	RPE         int
	RestSeconds int
}

func NewWorkoutPrescription(sets, reps int, weight float64, rpe, restSeconds int) (WorkoutPrescription, error) {
	if sets <= 0 || reps <= 0 {
		return WorkoutPrescription{}, fmt.Errorf("%w: sets and reps must be positive", ErrInvalidPrescription)
	}
	if weight < 0 {
		return WorkoutPrescription{}, fmt.Errorf("%w: weight cannot be negative", ErrInvalidPrescription)
	}
	if rpe < 1 || rpe > 10 {
		return WorkoutPrescription{}, fmt.Errorf("%w: RPE must be between 1 and 10", ErrInvalidPrescription)
	}
	return WorkoutPrescription{
		Sets:        sets,
		Reps:        reps,
		Weight:      weight,
		RPE:         rpe,
		RestSeconds: restSeconds,
	}, nil
}
