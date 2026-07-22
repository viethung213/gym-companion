package domain

import "fmt"

// WorkoutPrescription represents the target exercise prescription (Sets, Reps, Weight, RPE, RestSeconds).
type WorkoutPrescription struct {
	sets        int
	reps        int
	weight      float64
	rpe         float64
	restSeconds int
}

// NewWorkoutPrescription creates and validates a new WorkoutPrescription value object.
func NewWorkoutPrescription(sets, reps int, weight, rpe float64, restSeconds int) (WorkoutPrescription, error) {
	wp := WorkoutPrescription{
		sets:        sets,
		reps:        reps,
		weight:      weight,
		rpe:         rpe,
		restSeconds: restSeconds,
	}

	if err := wp.Validate(); err != nil {
		return WorkoutPrescription{}, err
	}

	return wp, nil
}

// Validate checks business invariant rules for WorkoutPrescription.
func (wp WorkoutPrescription) Validate() error {
	if wp.sets <= 0 {
		return fmt.Errorf("%w: sets must be greater than zero", ErrInvalidPrescription)
	}
	if wp.reps <= 0 {
		return fmt.Errorf("%w: reps must be greater than zero", ErrInvalidPrescription)
	}
	if wp.weight < 0 {
		return fmt.Errorf("%w: weight cannot be negative", ErrInvalidPrescription)
	}
	if wp.rpe < 0 || wp.rpe > 10 {
		return fmt.Errorf("%w: rpe must be between 0 and 10", ErrInvalidPrescription)
	}
	if wp.restSeconds < 0 {
		return fmt.Errorf("%w: rest seconds cannot be negative", ErrInvalidPrescription)
	}

	return nil
}

func (wp WorkoutPrescription) Sets() int {
	return wp.sets
}

func (wp WorkoutPrescription) Reps() int {
	return wp.reps
}

func (wp WorkoutPrescription) Weight() float64 {
	return wp.weight
}

func (wp WorkoutPrescription) RPE() float64 {
	return wp.rpe
}

func (wp WorkoutPrescription) RestSeconds() int {
	return wp.restSeconds
}

