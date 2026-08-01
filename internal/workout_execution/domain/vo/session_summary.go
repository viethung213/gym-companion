package vo

// SessionSummary represents the aggregated statistics of a completed workout session.
type SessionSummary struct {
	TotalSets        int
	TotalVolume      float32
	AverageFormScore *float32 // nil for non-AI sessions
	AverageRPE       float32
}

// NewSessionSummary constructs a new immutable SessionSummary.
func NewSessionSummary(
	totalSets int,
	totalVolume float32,
	averageFormScore *float32,
	averageRPE float32,
) SessionSummary {
	var formScoreCopy *float32
	if averageFormScore != nil {
		val := *averageFormScore
		formScoreCopy = &val
	}

	return SessionSummary{
		TotalSets:        totalSets,
		TotalVolume:      totalVolume,
		AverageFormScore: formScoreCopy,
		AverageRPE:       averageRPE,
	}
}
