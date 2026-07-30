package event

import (
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// WorkoutSessionStarted is published when a new workout session is initiated.

type WorkoutSessionStarted struct {
	SessionID string `json:"sessionId"`

	UserID string `json:"userId"`

	PlanID string `json:"planId"`

	StartedAt time.Time `json:"startedAt"`
}

// EventName returns the CloudEvent type for WorkoutSessionStarted.

func (e WorkoutSessionStarted) EventName() string {

	return "contracts.core.workout_execution.v1.workoutSessionStarted"

}

// WorkoutSessionCompleted is published when a workout session finishes normally.

type WorkoutSessionCompleted struct {
	SessionID string `json:"sessionId"`

	UserID string `json:"userId"`

	PlanID string `json:"planId"`

	CompletedAt time.Time `json:"completedAt"`

	Summary vo.SessionSummary `json:"summary"`
}

// EventName returns the CloudEvent type for WorkoutSessionCompleted.

func (e WorkoutSessionCompleted) EventName() string {

	return "contracts.core.workout_execution.v1.workoutSessionCompleted"

}

// WorkoutSessionAborted is published when a workout session is aborted or marked anomalous.

type WorkoutSessionAborted struct {
	SessionID string `json:"sessionId"`

	UserID string `json:"userId"`

	PlanID string `json:"planId"`

	Reason string `json:"reason"`

	IsAnomalous bool `json:"isAnomalous"`

	AbortedAt time.Time `json:"abortedAt"`
}

// EventName returns the CloudEvent type for WorkoutSessionAborted.

func (e WorkoutSessionAborted) EventName() string {

	return "contracts.core.workout_execution.v1.workoutSessionAborted"

}

// NewPersonalRecordAchieved is published when a user beats their previous 1RM record.

type NewPersonalRecordAchieved struct {
	UserID string `json:"userId"`

	ExerciseID string `json:"exerciseId"`

	OneRepMax float32 `json:"oneRepMax"`

	Weight float32 `json:"weight"`

	Reps int `json:"reps"`

	FormVerified bool `json:"formVerified"`

	AchievedAt time.Time `json:"achievedAt"`
}

// EventName returns the CloudEvent type for NewPersonalRecordAchieved.

func (e NewPersonalRecordAchieved) EventName() string {

	return "contracts.core.workout_execution.v1.newPersonalRecordAchieved"

}

// BodyMetricUpdated is published if user updates weight at the end of a session.

type BodyMetricUpdated struct {
	UserID string `json:"userId"`

	WeightKg float32 `json:"weightKg"`

	RecordedAt time.Time `json:"recordedAt"`
}

// EventName returns the CloudEvent type for BodyMetricUpdated.

func (e BodyMetricUpdated) EventName() string {

	return "contracts.core.workout_execution.v1.bodyMetricUpdated"

}

// MotionSpecificationUpdated is published when an exercise's ONNX model or posture rules are updated.

type MotionSpecificationUpdated struct {
	ExerciseID string `json:"exerciseId"`

	OnnxModelURL string `json:"onnxModelUrl"`

	LocalRulesURL string `json:"localRulesUrl"`

	DialogueEngineURL string `json:"dialogueEngineUrl"`

	RecommendedCameraAngle string `json:"recommendedCameraAngle"`

	IsReady bool `json:"isReady"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// EventName returns the CloudEvent type for MotionSpecificationUpdated.

func (e MotionSpecificationUpdated) EventName() string {

	return "contracts.core.workout_execution.v1.motionSpecificationUpdated"

}
