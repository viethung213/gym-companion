package valueobject

import "github.com/viethung213/gym-companion/internal/solo_match/domain/derror"

type MatchResult string

const (
	MatchResultUnknown   MatchResult = "UNKNOWN"
	MatchResultPending   MatchResult = "PENDING"
	MatchResultWin       MatchResult = "WIN"
	MatchResultLose      MatchResult = "LOSE"
	MatchResultDraw      MatchResult = "DRAW"
	MatchResultForfeited MatchResult = "FORFEITED"
)

func NewMatchResult(s string) (MatchResult, error) {
	res := MatchResult(s)
	if !res.IsValid() {
		return MatchResultUnknown, derror.ErrInvalidMatchResult
	}
	return res, nil
}

func (r MatchResult) String() string {
	return string(r)
}

func (r MatchResult) IsValid() bool {
	switch r {
	case MatchResultPending, MatchResultWin, MatchResultLose, MatchResultDraw, MatchResultForfeited:
		return true
	default:
		return false
	}
}
