package vo

// CalibrationConfig holds distance and device angle calibration thresholds for AI camera tracking.
type CalibrationConfig struct {
	MinDistanceMeters float32 `json:"minDistanceMeters"`
	MaxDistanceMeters float32 `json:"maxDistanceMeters"`
	TargetAngle       float32 `json:"targetAngle"`
}

// RepCountingRules holds motion parameters for determining valid reps.
type RepCountingRules struct {
	MinROMPercentage float32 `json:"minRomPercentage"` // e.g. 70.0%
}

// FormScoringRules holds rules for evaluating posture correctness.
type FormScoringRules struct {
	PenaltyPerError float32 `json:"penaltyPerError"`
}

// DialogueOption defines a voice/text feedback line for AI coach.
type DialogueOption struct {
	Text     string `json:"text"`
	AudioURL string `json:"audioUrl"`
}

// DialogueSeverities maps severity levels to dialogue options.
type DialogueSeverities struct {
	Severity1 []DialogueOption `json:"severity1"`
	Severity2 []DialogueOption `json:"severity2"`
}

// DialogueEngineConfig holds audio feedback rules and cooldowns for a specific coach personality.
type DialogueEngineConfig struct {
	PersonalityID string                         `json:"personalityId"`
	Cooldowns     map[string]float32             `json:"cooldowns"`
	DialogueMap   map[string]DialogueSeverities `json:"dialogueMap"`
}
