package domain

import "time"

type Event struct {
	ID           string
	Type         string
	PartitionKey string
	Payload      []byte
	CreatedAt    time.Time
}

const (
	EventTypeExerciseCreated              = "contracts.supporting.exercise.v1.exerciseCreated"
	EventTypeExerciseSubmittedForApproval = "contracts.supporting.exercise.v1." +
		"exerciseSubmittedForApproval"
	EventTypeExerciseApproved = "contracts.supporting.exercise.v1.exerciseApproved"
	EventTypeExerciseArchived = "contracts.supporting.exercise.v1.exerciseArchived"
)
