package port

import (
	"context"
	"errors"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// ErrExerciseNotFound is returned by ExerciseCatalogReader.GetByID when the
// given exercise_id does not exist in the catalog. Callers must distinguish
// this from transport failures: a missing ID is a defect in the data being
// validated, whereas any other error means the catalog is unreachable and the
// caller cannot conclude anything about the ID's validity.
var ErrExerciseNotFound = errors.New("exercise not found")

// Slot represents a user's available training window.
type Slot struct {
	DayOfWeek time.Weekday
	StartTime string // "HH:MM"
	EndTime   string // "HH:MM"
}

// InjuryStatus is a compact summary of one injury.
type InjuryStatus struct {
	Region      taxonomy.JointRegion
	Muscle      taxonomy.MuscleGroup // optional
	Severity    taxonomy.InjurySeverity
	ReportedAt  time.Time
	RecoveredAt *time.Time
	Notes       string
}

// Injury returns the eligibility view of the injury.
func (s *InjuryStatus) Injury() service.Injury {
	return service.Injury{Region: s.Region, Muscle: s.Muscle, Severity: s.Severity}
}

// Profile is what Coaching needs from the Profile bounded context.
type Profile struct {
	UserID                string
	WeightKg              float64
	HeightCm              float64
	Age                   int
	ExperienceLevel       string
	PrimaryGoal           string
	AvailableEquipment    []taxonomy.Equipment
	PreferredMuscleGroups []taxonomy.MuscleGroup
	AvailableSlots        []Slot
	// Location is the athlete's zone; slot times are wall clock in it. Read it via Zone.
	Location       *time.Location
	ActiveInjuries []InjuryStatus
	UpdatedAt      time.Time
}

// Athlete returns the eligibility view of the profile: owned equipment,
// highest exercise level and the injuries not yet recovered.
func (p *Profile) Athlete() service.Athlete {
	athlete := service.Athlete{
		Equipment: append([]taxonomy.Equipment(nil), p.AvailableEquipment...),
		Level:     service.UserLevel(p.ExperienceLevel),
	}
	for i := range p.ActiveInjuries {
		if p.ActiveInjuries[i].RecoveredAt == nil {
			athlete.Injuries = append(athlete.Injuries, p.ActiveInjuries[i].Injury())
		}
	}
	return athlete
}

// Zone returns the athlete's time zone, UTC when unknown.
func (p *Profile) Zone() *time.Location {
	if p.Location == nil {
		return time.UTC
	}
	return p.Location
}

// UserProfileReader is the port for querying the Profile bounded context.
type UserProfileReader interface {
	GetProfile(ctx context.Context, userID string) (Profile, error)
}

// WorkoutSession is the subset of workout execution data Coaching needs.
type WorkoutSession struct {
	SessionID        string
	UserID           string
	PlanID           string
	CompletedAt      time.Time
	TotalSets        int
	TotalPrescribed  int
	AverageRPE       float64
	AverageFormScore float64
	Aborted          bool
	AbortReason      string
}

// PersonalRecordSummary is a compact PR record for one exercise.
type PersonalRecordSummary struct {
	ExerciseID string
	OneRepMax  float64
	Weight     float64
	Reps       int
	AchievedAt time.Time
}

// WorkoutSessionReader is the port for querying execution history.
type WorkoutSessionReader interface {
	GetRecentSessions(ctx context.Context, userID string, since time.Time) ([]WorkoutSession, error)
	GetPersonalRecords(ctx context.Context, userID string) ([]PersonalRecordSummary, error)
}

// ExerciseFilter is the catalog search filter, mapped onto the Exercise
// SearchExercisesRequest. Slice filters match any listed value;
// ExcludeMuscles drops exercises loading a listed muscle as primary or secondary.
type ExerciseFilter struct {
	PrimaryMuscles []taxonomy.MuscleGroup
	Equipment      []taxonomy.Equipment
	Categories     []taxonomy.Category
	Levels         []taxonomy.Level
	ExcludeMuscles []taxonomy.MuscleGroup
	Keyword        string
	Limit          int
	Offset         int
}

// Exercise is the minimal exercise info Coaching consumes.
type Exercise struct {
	ExerciseID       string
	Name             string
	PrimaryMuscle    taxonomy.MuscleGroup
	SecondaryMuscles []taxonomy.MuscleGroup
	Equipment        taxonomy.Equipment
	Category         taxonomy.Category
	Level            taxonomy.Level
	Force            taxonomy.Force
	Mechanic         taxonomy.Mechanic
	MeasurementType  string // "REPS" | "DURATION"
}

// Traits returns the eligibility view of the exercise.
// IsTimed reports whether the exercise is held or timed rather than counted.
func (e *Exercise) IsTimed() bool { return e.MeasurementType == "DURATION" }

func (e *Exercise) Traits() service.ExerciseTraits {
	return service.ExerciseTraits{
		PrimaryMuscle:    e.PrimaryMuscle,
		SecondaryMuscles: append([]taxonomy.MuscleGroup(nil), e.SecondaryMuscles...),
		Equipment:        e.Equipment,
		Category:         e.Category,
		Level:            e.Level,
		Force:            e.Force,
		Mechanic:         e.Mechanic,
	}
}

// ExerciseCatalogReader is the port for querying the Exercise catalog.
type ExerciseCatalogReader interface {
	SearchByFilter(ctx context.Context, f *ExerciseFilter) ([]Exercise, error)
	GetByID(ctx context.Context, exerciseID string) (Exercise, error)
}

// Notifier delivers personality-styled messages to the user. Phase-1 uses a
// stub logger implementation.
type Notifier interface {
	Send(ctx context.Context, userID string, message string, promptID string) error
}

// UserResponse is what UserResponseWaiter returns when the user replies.
type UserResponse struct {
	Choice string // opaque option code chosen by the user
}

// UserResponseWaiter blocks (with timeout) waiting for the user to reply to a
// prompt sent by Notifier. Phase-1 uses an in-memory dev stub.
type UserResponseWaiter interface {
	Wait(ctx context.Context, promptID string, timeout time.Duration) (UserResponse, error)
}
