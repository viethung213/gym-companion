package vo

import (
	"math"
)

const (
	// MaxLevel defines the highest attainable level in the platform.
	MaxLevel = int32(100)

	// LevelFactor is the scaling coefficient in the square-root progression formula.
	LevelFactor = float64(50)
)

// CalculateLevel computes the lifetime level [1..100] for a given cumulative XP.
// Formula: Level(X) = min(floor(sqrt(X / 50)) + 1, 100)
func CalculateLevel(xp int64) int32 {
	if xp <= 0 {
		return 1
	}

	rawLevel := int32(math.Floor(math.Sqrt(float64(xp)/LevelFactor))) + 1
	if rawLevel > MaxLevel {
		return MaxLevel
	}
	if rawLevel < 1 {
		return 1
	}
	return rawLevel
}

// XpRequiredForLevel calculates the minimum lifetime XP threshold to attain a given level.
// Inverse formula: XP(L) = 50 * (L - 1)^2
func XpRequiredForLevel(level int32) int64 {
	if level <= 1 {
		return 0
	}
	if level > MaxLevel {
		level = MaxLevel
	}
	diff := int64(level - 1)
	return int64(LevelFactor) * diff * diff
}

// XpToNextLevel computes the remaining XP needed to progress to currentLevel + 1.
// If the user is already at MaxLevel (100), it returns 0.
func XpToNextLevel(xp int64) int64 {
	currentLevel := CalculateLevel(xp)
	if currentLevel >= MaxLevel {
		return 0
	}

	nextLevelThreshold := XpRequiredForLevel(currentLevel + 1)
	if xp >= nextLevelThreshold {
		return 0
	}
	return nextLevelThreshold - xp
}

// ProgressPercentage calculates the completion percentage [0.0..100.0] within the current level bracket.
func ProgressPercentage(xp int64) float64 {
	currentLevel := CalculateLevel(xp)
	if currentLevel >= MaxLevel {
		return 100.0
	}

	currentBase := XpRequiredForLevel(currentLevel)
	nextBase := XpRequiredForLevel(currentLevel + 1)
	bracketSpan := nextBase - currentBase
	if bracketSpan <= 0 {
		return 100.0
	}

	progress := float64(xp-currentBase) / float64(bracketSpan) * 100.0
	if progress < 0.0 {
		return 0.0
	}
	if progress > 100.0 {
		return 100.0
	}
	return progress
}
