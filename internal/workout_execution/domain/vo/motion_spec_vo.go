package vo

// CalibrationConfig holds distance and device angle calibration thresholds for AI camera tracking.
type CalibrationConfig struct {
	MinDistanceMeters float32
	MaxDistanceMeters float32
	TargetAngle       float32
}

// RepCountingRules holds motion parameters for determining valid reps.
type RepCountingRules struct {
	MinROMPercentage float32 // e.g. 70.0%
}

// FormScoringRules holds rules for evaluating posture correctness.
type FormScoringRules struct {
	PenaltyPerError float32
}
