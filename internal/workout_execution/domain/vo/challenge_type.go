package vo

import "strings"

// ChallengeType represents the type of solo workout challenge.
type ChallengeType string

const (
	ChallengeTypeUnspecified ChallengeType = ""
	ChallengeTypeReps        ChallengeType = "REPS"
	ChallengeTypeDuration    ChallengeType = "DURATION"
)

// ParseChallengeType normalizes a string representation into ChallengeType.
func ParseChallengeType(raw string) ChallengeType {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DURATION", "SOLO_CHALLENGE_TYPE_DURATION":
		return ChallengeTypeDuration
	case "REPS", "SOLO_CHALLENGE_TYPE_REPS":
		return ChallengeTypeReps
	default:
		return ChallengeTypeUnspecified
	}
}

// IsValid reports whether the challenge type is known and supported.
func (c ChallengeType) IsValid() bool {
	return c == ChallengeTypeReps || c == ChallengeTypeDuration
}

// String returns the string representation of the ChallengeType.
func (c ChallengeType) String() string {
	return string(c)
}
