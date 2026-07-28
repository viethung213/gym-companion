package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
)

// Calculate1RMEpley computes estimated 1-Rep Max using the Epley formula: 1RM = W * (1 + R / 30).
func Calculate1RMEpley(weight float32, reps int) float32 {
	if reps <= 0 || weight <= 0 {
		return 0
	}
	if reps == 1 {
		return weight
	}
	return weight * (1.0 + float32(reps)/30.0)
}

// PersonalRecord is the aggregate root tracking 1RM records per user & exercise.
type PersonalRecord struct {
	id           string
	userID       string
	exerciseID   string
	oneRepMax    float32
	weight       float32
	reps         int
	formVerified bool
	achievedAt   time.Time
	createdAt    time.Time
	updatedAt    time.Time
	domainEvents []interface{}
}

// NewPersonalRecord constructs a new PersonalRecord instance.
func NewPersonalRecord(
	id, userID, exerciseID string,
	weight float32,
	reps int,
	formVerified bool,
	achievedAt time.Time,
) *PersonalRecord {
	oneRM := Calculate1RMEpley(weight, reps)
	now := time.Now().UTC()
	if achievedAt.IsZero() {
		achievedAt = now
	}

	pr := &PersonalRecord{
		id:           id,
		userID:       userID,
		exerciseID:   exerciseID,
		oneRepMax:    oneRM,
		weight:       weight,
		reps:         reps,
		formVerified: formVerified,
		achievedAt:   achievedAt,
		createdAt:    now,
		updatedAt:    now,
	}

	pr.addDomainEvent(event.NewPersonalRecordAchieved{
		UserID:       userID,
		ExerciseID:   exerciseID,
		OneRepMax:    oneRM,
		Weight:       weight,
		Reps:         reps,
		FormVerified: formVerified,
		AchievedAt:   achievedAt,
	})

	return pr
}

// ReconstitutePersonalRecord restores PersonalRecord state from database.
func ReconstitutePersonalRecord(
	id, userID, exerciseID string,
	oneRepMax, weight float32,
	reps int,
	formVerified bool,
	achievedAt, createdAt, updatedAt time.Time,
) *PersonalRecord {
	return &PersonalRecord{
		id:           id,
		userID:       userID,
		exerciseID:   exerciseID,
		oneRepMax:    oneRepMax,
		weight:       weight,
		reps:         reps,
		formVerified: formVerified,
		achievedAt:   achievedAt,
		createdAt:    createdAt,
		updatedAt:    updatedAt,
	}
}

// ID returns PR ID.
func (pr *PersonalRecord) ID() string { return pr.id }

// UserID returns User ID.
func (pr *PersonalRecord) UserID() string { return pr.userID }

// ExerciseID returns Exercise ID.
func (pr *PersonalRecord) ExerciseID() string { return pr.exerciseID }

// OneRepMax returns estimated 1RM.
func (pr *PersonalRecord) OneRepMax() float32 { return pr.oneRepMax }

// Weight returns weight.
func (pr *PersonalRecord) Weight() float32 { return pr.weight }

// Reps returns reps.
func (pr *PersonalRecord) Reps() int { return pr.reps }

// FormVerified returns form verification status.
func (pr *PersonalRecord) FormVerified() bool { return pr.formVerified }

// AchievedAt returns achievement timestamp.
func (pr *PersonalRecord) AchievedAt() time.Time { return pr.achievedAt }

// PopEvents returns and clears pending domain events.
func (pr *PersonalRecord) PopEvents() []interface{} {
	events := pr.domainEvents
	pr.domainEvents = nil
	return events
}

func (pr *PersonalRecord) addDomainEvent(ev interface{}) {
	pr.domainEvents = append(pr.domainEvents, ev)
}

// UpdateIfHigher evaluates new performance and updates 1RM if strictly higher.
func (pr *PersonalRecord) UpdateIfHigher(weight float32, reps int, formVerified bool, achievedAt time.Time) bool {
	new1RM := Calculate1RMEpley(weight, reps)
	if new1RM <= pr.oneRepMax {
		return false
	}

	now := time.Now().UTC()
	if achievedAt.IsZero() {
		achievedAt = now
	}

	pr.oneRepMax = new1RM
	pr.weight = weight
	pr.reps = reps
	pr.formVerified = formVerified
	pr.achievedAt = achievedAt
	pr.updatedAt = now

	pr.addDomainEvent(event.NewPersonalRecordAchieved{
		UserID:       pr.userID,
		ExerciseID:   pr.exerciseID,
		OneRepMax:    new1RM,
		Weight:       weight,
		Reps:         reps,
		FormVerified: formVerified,
		AchievedAt:   achievedAt,
	})

	return true
}
