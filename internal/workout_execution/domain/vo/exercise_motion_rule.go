package vo

import "time"

// FormulaType defines the type of calculation in a rule criterion.
type FormulaType string

const (
	FormulaAngle3D  FormulaType = "ANGLE_3D"
	FormulaDistance FormulaType = "DISTANCE"
)

// ValidAngleRange specifies minimum and maximum valid angles in degrees.
type ValidAngleRange struct {
	Min float32
	Max float32
}

// RuleCriterion defines a single calculation and its valid threshold criteria.
type RuleCriterion struct {
	Name            string
	Formula         FormulaType
	Joints          []int
	ValidRange      ValidAngleRange
	ErrorCodeBelow  string
	ErrorCodeAbove  string
	AudioCue        string
	PenaltyPerError float32
	ApplyForView    string
}

// ThresholdCondition defines comparison operators (gt, gte, lt, lte) for a phase boundary.
type ThresholdCondition struct {
	GT      float32 `json:"gt,omitempty"`
	GTE     float32 `json:"gte,omitempty"`
	LT      float32 `json:"lt,omitempty"`
	LTE     float32 `json:"lte,omitempty"`
	Comment string  `json:"comment,omitempty"`
}

// Matches checks whether a given angle satisfies the threshold condition.
func (c ThresholdCondition) Matches(val float32) bool {
	if c.GT > 0 && val <= c.GT {
		return false
	}
	if c.GTE > 0 && val < c.GTE {
		return false
	}
	if c.LT > 0 && val >= c.LT {
		return false
	}
	if c.LTE > 0 && val > c.LTE {
		return false
	}
	return true
}

// PhaseThresholds defines the thresholds for rest, transition, and active phases (or idle/always for timed exercises).
type PhaseThresholds struct {
	Rest       ThresholdCondition `json:"rest,omitempty"`
	Transition ThresholdCondition `json:"transition,omitempty"`
	Active     ThresholdCondition `json:"active,omitempty"`
	Idle       ThresholdCondition `json:"idle,omitempty"`
	Always     ThresholdCondition `json:"always,omitempty"`
}

// PhaseCalculation specifies the landmarks and formula used to calculate the phase angle.
type PhaseCalculation struct {
	Landmarks   []string    `json:"landmarks,omitempty"`
	Joints      []int       `json:"joints,omitempty"`
	Formula     FormulaType `json:"formula,omitempty"`
	Description string      `json:"description,omitempty"`
}

// PhaseDetectionConfig encapsulates the declarative configuration for rep lifecycle tracking.
type PhaseDetectionConfig struct {
	Metric      string           `json:"metric"`
	Thresholds  PhaseThresholds  `json:"thresholds"`
	Calculation PhaseCalculation `json:"calculation"`
}

// ExerciseMotionRule encapsulates the declarative specification for evaluating exercise form.
type ExerciseMotionRule struct {
	ExerciseID             string
	ExerciseName           string
	DisplayName            string
	ChallengeType          ChallengeType
	RepType                string // "counted" or "timed"
	Notes                  string
	MinConfidence          float32
	PhaseDetection         *PhaseDetectionConfig
	Rules                  []RuleCriterion
	PassCondition          string // "ALL_MATCH"
	InflectionThreshold    float32
	ReturnThreshold        float32
	TimeCeiling            time.Duration
	ViolationMaxDuration   time.Duration
	RecommendedCameraAngle string
}

// SkeletonPoint represents a single 3D joint landmark normalized coordinate.
type SkeletonPoint struct {
	Index      int
	X          float32
	Y          float32
	Z          float32
	Visibility float32
	Presence   float32
}

// EvaluationResult captures the outcome of evaluating a frame against an ExerciseMotionRule.
type EvaluationResult struct {
	Passed          bool
	FormScore       float32
	DetectedErrors  []string
	AudioCue        string
	CalculatedAngle float32
}
