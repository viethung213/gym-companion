package valueobject

import "github.com/viethung213/gym-companion/internal/solo_match/domain/derror"

type ParticipantScore struct {
	value float64
}

func NewParticipantScore(value float64) (ParticipantScore, error) {
	if value < 0 {
		return ParticipantScore{}, derror.ErrNegativeScore
	}
	return ParticipantScore{value: value}, nil
}

func (s ParticipantScore) Value() float64 {
	return s.value
}
