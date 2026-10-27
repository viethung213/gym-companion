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

// WorkoutSessionShared is published when a completed workout session is shared to the social feed.
type WorkoutSessionShared struct {
	SessionID       string
	UserID          string
	Caption         string
	MediaURLs       []string
	Visibility      string
	DurationSeconds int32
	TotalSets       int32
	TotalVolumeKg   float32
	SharedAt        time.Time
	WorkoutTitle    string
	ExerciseCount   int32
}

// EventName returns the CloudEvent type for WorkoutSessionShared.
func (e *WorkoutSessionShared) EventName() string {
	return "contracts.core.workout_execution.v1.workoutSessionShared"
}

// SoloTrackingCompleted is published when a solo match tracking session ends.
type SoloTrackingCompleted struct {
	MatchID              string
	UserID               string
	ExerciseID           string
	ChallengeType        vo.ChallengeType
	ConfirmedValue       int32
	AverageFormScore     float32
	TotalDurationSeconds int32
	CompletedAt          time.Time
	PlanID               string
}

// EventName returns the CloudEvent type for SoloTrackingCompleted.
func (e *SoloTrackingCompleted) EventName() string {
	return "contracts.core.workout_execution.v1.soloTrackingCompleted"
}

// SoloTrackingAborted is published when a solo challenge session is aborted or forfeited midway.
type SoloTrackingAborted struct {
	MatchID              string
	UserID               string
	ExerciseID           string
	Reason               string
	AbortedAt            time.Time
	PlanID               string
	ConfirmedValue       int32
	TotalDurationSeconds int32
}

// EventName returns the CloudEvent type for SoloTrackingAborted.
func (e *SoloTrackingAborted) EventName() string {
	return "contracts.core.workout_execution.v1.soloTrackingAborted"
}

// SoloMatchRoomReady is published when both participants have joined the match room and are ready.
type SoloMatchRoomReady struct {
	MatchID   string
	StartedAt time.Time
}

func (e *SoloMatchRoomReady) EventName() string {
	return "contracts.core.workout_execution.v1.soloMatchRoomReady"
}
