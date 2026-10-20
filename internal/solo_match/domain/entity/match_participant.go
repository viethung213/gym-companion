package entity

import (
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type MatchParticipant struct {
	id          string
	matchID     string
	userID      string
	score       valueobject.ParticipantScore
	isCompleted bool
	completedAt *time.Time
	result      valueobject.MatchResult
}

func NewMatchParticipant(
	id string,
	matchID string,
	userID string,
) (*MatchParticipant, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, derror.ErrEmptyParticipantID
	}
	trimmedMatchID := strings.TrimSpace(matchID)
	if trimmedMatchID == "" {
		return nil, derror.ErrEmptyMatchID
	}
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return nil, derror.ErrEmptyUserID
	}

	zeroScore, _ := valueobject.NewParticipantScore(0)

	return &MatchParticipant{
		id:          trimmedID,
		matchID:     trimmedMatchID,
		userID:      trimmedUserID,
		score:       zeroScore,
		isCompleted: false,
		completedAt: nil,
		result:      valueobject.MatchResultPending,
	}, nil
}

// ReconstituteParticipant khôi phục entity từ persistence
func ReconstituteParticipant(
	id string,
	matchID string,
	userID string,
	score valueobject.ParticipantScore,
	isCompleted bool,
	completedAt *time.Time,
	result valueobject.MatchResult,
) *MatchParticipant {
	return &MatchParticipant{
		id:          id,
		matchID:     matchID,
		userID:      userID,
		score:       score,
		isCompleted: isCompleted,
		completedAt: completedAt,
		result:      result,
	}
}

func (p *MatchParticipant) ID() string {
	return p.id
}

func (p *MatchParticipant) MatchID() string {
	return p.matchID
}

func (p *MatchParticipant) UserID() string {
	return p.userID
}

func (p *MatchParticipant) Score() valueobject.ParticipantScore {
	return p.score
}

func (p *MatchParticipant) IsCompleted() bool {
	return p.isCompleted
}

func (p *MatchParticipant) CompletedAt() *time.Time {
	if p.completedAt == nil {
		return nil
	}
	t := *p.completedAt
	return &t
}

func (p *MatchParticipant) Result() valueobject.MatchResult {
	return p.result
}

// RecordResult ghi nhận điểm số khi hoàn thành bài tập
func (p *MatchParticipant) RecordResult(scoreVal float64, completedAt time.Time) error {
	if p.isCompleted {
		return derror.ErrParticipantFinished
	}
	score, err := valueobject.NewParticipantScore(scoreVal)
	if err != nil {
		return fmt.Errorf("invalid score: %w", err)
	}

	p.score = score
	p.isCompleted = true
	t := completedAt.UTC()
	p.completedAt = &t
	return nil
}

// Forfeit đánh dấu người chơi bỏ cuộc
func (p *MatchParticipant) Forfeit() {
	p.result = valueobject.MatchResultForfeited
	p.isCompleted = true
	if p.completedAt == nil {
		t := time.Now().UTC()
		p.completedAt = &t
	}
}

func (p *MatchParticipant) SetResult(res valueobject.MatchResult) {
	p.result = res
}
