package port

import (
	"context"
	"time"
)

// Slot represents a user's available training window.
type Slot struct {
	DayOfWeek time.Weekday
	StartTime string // "HH:MM"
	EndTime   string // "HH:MM"
}

// InjuryStatus is a compact summary of one active injury.
type InjuryStatus struct {
	MuscleGroup string
	ReportedAt  time.Time
	RecoveredAt *time.Time
	Notes       string
}

// Profile is what Coaching needs from the Profile bounded context.
type Profile struct {
	UserID                string
	WeightKg              float64
	HeightCm              float64
	Age                   int
	PrimaryGoal           string
	AvailableEquipment    []string
	PreferredMuscleGroups []string
	AvailableSlots        []Slot
	ActiveInjuries        []InjuryStatus
	UpdatedAt             time.Time
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

// SetLog is a minimal set-level record used by 1RM estimation.
type SetLog struct {
	ExerciseID  string
	Weight      float64
	Reps        int
	RPE         float64
	CompletedAt time.Time
}

// WorkoutSessionReader is the port for querying execution history.
type WorkoutSessionReader interface {
	GetRecentSessions(ctx context.Context, userID string, since time.Time) ([]WorkoutSession, error)
	GetSetLogs(ctx context.Context, userID string, exerciseID string, limit int) ([]SetLog, error)
}

// ExerciseFilter is the search filter mapped directly from Exercise gRPC contract SearchExercisesRequest.
type ExerciseFilter struct {
	BodyPartID         string
	TargetMuscleID     string
	SecondaryMuscleIDs []string
	EquipmentIDs       []string
	AvoidInjuryAreas   []string
	TagIDs             []string
	Keyword            string
	Difficulty         string
	Limit              int
	Offset             int
}

// Exercise is the minimal exercise info Coaching consumes.
type Exercise struct {
	ExerciseID       string
	Name             string
	MuscleGroup      string
	Equipment        string
	Difficulty       string
	IsBodyweight     bool
	IsMachineOrCable bool
}

// ExerciseCatalogReader is the port for querying the Exercise catalog.
type ExerciseCatalogReader interface {
	SearchByFilter(ctx context.Context, f ExerciseFilter) ([]Exercise, error)
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
