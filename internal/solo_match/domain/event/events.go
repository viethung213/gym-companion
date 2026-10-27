package event

import (
	"time"
)

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

// SoloMatchStarted phát ra khi trận đấu bắt đầu
type SoloMatchStarted struct {
	MatchID       string
	ExerciseID    string
	ExerciseName  string
	ChallengeType string
	Player1ID     string
	Player2ID     string
	BotUserID     string // Rỗng nếu không có bot
	StartedAt     time.Time
}

func (e *SoloMatchStarted) EventName() string {
	return "contracts.core.solo_match.v1.soloMatchStarted"
}

func (e *SoloMatchStarted) OccurredAt() time.Time {
	return e.StartedAt
}

// SoloMatchCompleted phát ra khi trận đấu kết thúc có kết quả
type SoloMatchCompleted struct {
	MatchID       string
	ExerciseID    string
	ChallengeType string
	WinnerID      string
	IsDraw        bool
	Player1ID     string
	Player1Score  float64
	Player2ID     string
	Player2Score  float64
	BotUserID     string // Rỗng nếu không có bot
	CompletedAt   time.Time
}

func (e *SoloMatchCompleted) EventName() string {
	return "contracts.core.solo_match.v1.soloMatchCompleted"
}

func (e *SoloMatchCompleted) OccurredAt() time.Time {
	return e.CompletedAt
}

// SoloMatchForfeited phát ra khi một bên bỏ cuộc hoặc timeout
type SoloMatchForfeited struct {
	MatchID       string
	ForfeitedByID string
	WinnerID      string
	BotUserID     string // Rỗng nếu không có bot
	Reason        string
	ForfeitedAt   time.Time
}

func (e *SoloMatchForfeited) EventName() string {
	return "contracts.core.solo_match.v1.soloMatchForfeited"
}

func (e *SoloMatchForfeited) OccurredAt() time.Time {
	return e.ForfeitedAt
}

// SoloMatchCancelled phát ra khi trận đấu bị hủy do quá hạn điểm danh vào phòng
type SoloMatchCancelled struct {
	MatchID     string
	Reason      string
	CancelledAt time.Time
}

func (e *SoloMatchCancelled) EventName() string {
	return "contracts.core.solo_match.v1.soloMatchCancelled"
}

func (e *SoloMatchCancelled) OccurredAt() time.Time {
	return e.CancelledAt
}
