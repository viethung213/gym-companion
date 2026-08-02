package event

import (
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// WorkoutSessionStarted is published when a new workout session is initiated.
type WorkoutSessionStarted struct {
	SessionID string
	UserID    string
	PlanID    string
	StartedAt time.Time
}

// EventName returns the CloudEvent type for WorkoutSessionStarted.
func (e *WorkoutSessionStarted) EventName() string {
	return "contracts.core.workout_execution.v1.workoutSessionStarted"
}

// WorkoutSessionCompleted is published when a workout session finishes normally.
type WorkoutSessionCompleted struct {
	SessionID   string
	UserID      string
	PlanID      string
	CompletedAt time.Time
	Summary     vo.SessionSummary
}

// EventName returns the CloudEvent type for WorkoutSessionCompleted.
func (e *WorkoutSessionCompleted) EventName() string {
	return "contracts.core.workout_execution.v1.workoutSessionCompleted"
}

// WorkoutSessionAborted is published when a workout session is aborted or marked anomalous.
type WorkoutSessionAborted struct {
	SessionID   string
	UserID      string
	PlanID      string
	Reason      string
	IsAnomalous bool
	AbortedAt   time.Time
}

// EventName returns the CloudEvent type for WorkoutSessionAborted.
func (e *WorkoutSessionAborted) EventName() string {
	return "contracts.core.workout_execution.v1.workoutSessionAborted"
}

// NewPersonalRecordAchieved is published when a user beats their previous 1RM record.
type NewPersonalRecordAchieved struct {
	UserID       string
	ExerciseID   string
	OneRepMax    float32
	Weight       float32
	Reps         int
	FormVerified bool
	AchievedAt   time.Time
}

// EventName returns the CloudEvent type for NewPersonalRecordAchieved.
func (e *NewPersonalRecordAchieved) EventName() string {
	return "contracts.core.workout_execution.v1.newPersonalRecordAchieved"
}

// BodyMetricUpdated is published if user updates weight at the end of a session.
type BodyMetricUpdated struct {
	UserID     string
	WeightKg   float32
	RecordedAt time.Time
}

// EventName returns the CloudEvent type for BodyMetricUpdated.
func (e *BodyMetricUpdated) EventName() string {
	return "contracts.core.workout_execution.v1.bodyMetricUpdated"
}

// MotionSpecificationUpdated is published when an exercise's ONNX model or posture rules are updated.
type MotionSpecificationUpdated struct {
	ExerciseID             string
	OnnxDetectorURL        string
	OnnxSkeletonURL        string
	LocalRulesURL          string
	DialogueEngineURL      string
	RecommendedCameraAngle string
	IsReady                bool
	UpdatedAt              time.Time
}

// EventName returns the CloudEvent type for MotionSpecificationUpdated.
func (e *MotionSpecificationUpdated) EventName() string {
	return "contracts.core.workout_execution.v1.motionSpecificationUpdated"
}
