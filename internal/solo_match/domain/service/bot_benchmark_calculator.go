package service

import (
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

const (
	DefaultBotReps     float64 = 5.0
	DefaultBotDuration float64 = 300.0 // 5 phút (300 giây)
)

type BotBenchmarkCalculator struct{}

func NewBotBenchmarkCalculator() *BotBenchmarkCalculator {
	return &BotBenchmarkCalculator{}
}

// CalculateBotScore tính điểm số của Bot dựa trên PR của người chơi hoặc fallback mặc định
func (c *BotBenchmarkCalculator) CalculateBotScore(
	cType valueobject.ChallengeType,
	userPR *float64,
) float64 {
	if userPR != nil && *userPR > 0 {
		return *userPR
	}

	if cType == valueobject.ChallengeTypeDuration {
		return DefaultBotDuration
	}

	return DefaultBotReps
}
