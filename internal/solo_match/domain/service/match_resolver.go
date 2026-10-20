package service

import (
	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type MatchResolver struct{}

func NewMatchResolver() *MatchResolver {
	return &MatchResolver{}
}

// ResolveOutcome phân định người thắng, người thua hoặc hòa
func (r *MatchResolver) ResolveOutcome(
	cType valueobject.ChallengeType,
	p1, p2 *entity.MatchParticipant,
) (winnerID string, isDraw bool, err error) {
	if p1 == nil || p2 == nil {
		return "", false, derror.ErrNilParticipant
	}
	if !p1.IsCompleted() || !p2.IsCompleted() {
		return "", false, derror.ErrParticipantsNotCompleted
	}

	score1 := p1.Score().Value()
	score2 := p2.Score().Value()

	switch cType {
	case valueobject.ChallengeTypeReps, valueobject.ChallengeTypeDuration:
		if score1 > score2 {
			p1.SetResult(valueobject.MatchResultWin)
			p2.SetResult(valueobject.MatchResultLose)
			return p1.UserID(), false, nil
		}
		if score2 > score1 {
			p1.SetResult(valueobject.MatchResultLose)
			p2.SetResult(valueobject.MatchResultWin)
			return p2.UserID(), false, nil
		}
		p1.SetResult(valueobject.MatchResultDraw)
		p2.SetResult(valueobject.MatchResultDraw)
		return "", true, nil
	default:
		return "", false, derror.ErrInvalidChallengeType
	}
}
