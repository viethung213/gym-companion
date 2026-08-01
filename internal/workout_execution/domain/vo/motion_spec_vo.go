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

// DialogueOption defines a voice/text feedback line for AI coach.
type DialogueOption struct {
	Text     string
	AudioURL string
}

// DialogueSeverities maps severity levels to dialogue options.
type DialogueSeverities struct {
	Severity1 []DialogueOption
	Severity2 []DialogueOption
}

// DialogueEngineConfig holds audio feedback rules and cooldowns for a specific coach personality.
type DialogueEngineConfig struct {
	PersonalityID string
	Cooldowns     map[string]float32
	DialogueMap   map[string]DialogueSeverities
}
