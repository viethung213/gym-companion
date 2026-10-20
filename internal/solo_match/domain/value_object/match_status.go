package valueobject

import "github.com/viethung213/gym-companion/internal/solo_match/domain/derror"

type MatchStatus string

const (
	MatchStatusUnknown    MatchStatus = "UNKNOWN"
	MatchStatusMatched    MatchStatus = "MATCHED"
	MatchStatusInProgress MatchStatus = "IN_PROGRESS"
	MatchStatusCompleted  MatchStatus = "COMPLETED"
	MatchStatusForfeited  MatchStatus = "FORFEITED"
	MatchStatusCancelled  MatchStatus = "CANCELLED"
)

func NewMatchStatus(s string) (MatchStatus, error) {
	status := MatchStatus(s)
	if !status.IsValid() {
		return MatchStatusUnknown, derror.ErrInvalidMatchStatus
	}
	return status, nil
}

func (s MatchStatus) String() string {
	return string(s)
}

func (s MatchStatus) IsValid() bool {
	switch s {
	case MatchStatusMatched, MatchStatusInProgress, MatchStatusCompleted, MatchStatusForfeited, MatchStatusCancelled:
		return true
	default:
		return false
	}
}

func (s MatchStatus) IsFinal() bool {
	return s == MatchStatusCompleted || s == MatchStatusForfeited || s == MatchStatusCancelled
}
