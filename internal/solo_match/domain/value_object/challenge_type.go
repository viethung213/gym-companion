package valueobject

import "github.com/viethung213/gym-companion/internal/solo_match/domain/derror"

type ChallengeType string

const (
	ChallengeTypeUnknown  ChallengeType = "UNKNOWN"
	ChallengeTypeReps     ChallengeType = "REPS"
	ChallengeTypeDuration ChallengeType = "DURATION"
)

func NewChallengeType(s string) (ChallengeType, error) {
	ct := ChallengeType(s)
	if !ct.IsValid() {
		return ChallengeTypeUnknown, derror.ErrInvalidChallengeType
	}
	return ct, nil
}

func (c ChallengeType) String() string {
	return string(c)
}

func (c ChallengeType) IsValid() bool {
	return c == ChallengeTypeReps || c == ChallengeTypeDuration
}
