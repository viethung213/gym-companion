package aggregate

import (
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/service"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

type SoloMatch struct {
	id            string
	exerciseID    string
	exerciseName  string
	challengeType valueobject.ChallengeType
	status        valueobject.MatchStatus
	participants  []*entity.MatchParticipant
	winnerID      *string
	isDraw        bool
	startedAt     time.Time
	timeoutAt     time.Time
	endedAt       *time.Time
	domainEvents  []event.DomainEvent
}

func NewSoloMatch(
	id string,
	exerciseID string,
	exerciseName string,
	challengeType valueobject.ChallengeType,
	timeoutDuration time.Duration,
	participants []*entity.MatchParticipant,
	now time.Time,
) (*SoloMatch, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, derror.ErrEmptyMatchID
	}
	if len(participants) != 2 {
		return nil, derror.ErrInvalidParticipantCount
	}
	if participants[0] == nil || participants[1] == nil {
		return nil, derror.ErrNilParticipant
	}

	started := now.UTC()
	timeout := started.Add(timeoutDuration)

	return &SoloMatch{
		id:            trimmedID,
		exerciseID:    strings.TrimSpace(exerciseID),
		exerciseName:  strings.TrimSpace(exerciseName),
		challengeType: challengeType,
		status:        valueobject.MatchStatusInProgress,
		participants:  participants,
		startedAt:     started,
		timeoutAt:     timeout,
	}, nil
}

// Reconstitute khôi phục aggregate từ database
func Reconstitute(
	id string,
	exerciseID string,
	exerciseName string,
	challengeType valueobject.ChallengeType,
	status valueobject.MatchStatus,
	participants []*entity.MatchParticipant,
	winnerID *string,
	isDraw bool,
	startedAt time.Time,
	timeoutAt time.Time,
	endedAt *time.Time,
) *SoloMatch {
	return &SoloMatch{
		id:            id,
		exerciseID:    exerciseID,
		exerciseName:  exerciseName,
		challengeType: challengeType,
		status:        status,
		participants:  participants,
		winnerID:      winnerID,
		isDraw:        isDraw,
		startedAt:     startedAt,
		timeoutAt:     timeoutAt,
		endedAt:       endedAt,
	}
}

func (m *SoloMatch) ID() string {
	return m.id
}

func (m *SoloMatch) ExerciseID() string {
	return m.exerciseID
}

func (m *SoloMatch) ExerciseName() string {
	return m.exerciseName
}

func (m *SoloMatch) ChallengeType() valueobject.ChallengeType {
	return m.challengeType
}

func (m *SoloMatch) Status() valueobject.MatchStatus {
	return m.status
}

func (m *SoloMatch) Participants() []*entity.MatchParticipant {
	return m.participants
}

func (m *SoloMatch) WinnerID() *string {
	if m.winnerID == nil {
		return nil
	}
	s := *m.winnerID
	return &s
}

func (m *SoloMatch) IsDraw() bool {
	return m.isDraw
}

func (m *SoloMatch) StartedAt() time.Time {
	return m.startedAt
}

func (m *SoloMatch) TimeoutAt() time.Time {
	return m.timeoutAt
}

func (m *SoloMatch) EndedAt() *time.Time {
	if m.endedAt == nil {
		return nil
	}
	t := *m.endedAt
	return &t
}

func (m *SoloMatch) DomainEvents() []event.DomainEvent {
	return m.domainEvents
}

func (m *SoloMatch) ClearDomainEvents() {
	m.domainEvents = nil
}

func (m *SoloMatch) AddEvent(ev event.DomainEvent) {
	if ev != nil {
		m.domainEvents = append(m.domainEvents, ev)
	}
}

// PullEvents trả về toàn bộ events và dọn sạch hàng đợi sự kiện
func (m *SoloMatch) PullEvents() []event.DomainEvent {
	events := m.domainEvents
	m.domainEvents = nil
	return events
}

// RecordParticipantResult ghi nhận kết quả bài tập của một bên
func (m *SoloMatch) RecordParticipantResult(
	userID string,
	scoreVal float64,
	completedAt time.Time,
	resolver *service.MatchResolver,
) error {
	if m.status != valueobject.MatchStatusInProgress {
		return derror.ErrMatchNotActive
	}

	var target *entity.MatchParticipant
	for _, p := range m.participants {
		if p.UserID() == userID {
			target = p
			break
		}
	}

	if target == nil {
		return derror.ErrParticipantNotFound
	}

	if err := target.RecordResult(scoreVal, completedAt); err != nil {
		return fmt.Errorf("record result failed: %w", err)
	}

	// Kiểm tra xem cả hai bên đã hoàn thành chưa
	if m.participants[0].IsCompleted() && m.participants[1].IsCompleted() {
		winner, isDraw, err := resolver.ResolveOutcome(m.challengeType, m.participants[0], m.participants[1])
		if err != nil {
			return fmt.Errorf("resolve outcome failed: %w", err)
		}

		m.status = valueobject.MatchStatusCompleted
		m.isDraw = isDraw
		ended := time.Now().UTC()
		m.endedAt = &ended

		if !isDraw && winner != "" {
			m.winnerID = &winner
		}
	}

	return nil
}

// ForfeitParticipant xử thua ngay lập tức cho người chơi bỏ cuộc hoặc ngắt kết nối (Fast Forfeit)
func (m *SoloMatch) ForfeitParticipant(forfeitedUserID, _ string) error {
	if m.status != valueobject.MatchStatusInProgress {
		return derror.ErrMatchNotActive
	}

	var forfeited *entity.MatchParticipant
	var opponent *entity.MatchParticipant

	for _, p := range m.participants {
		if p.UserID() == forfeitedUserID {
			forfeited = p
		} else {
			opponent = p
		}
	}

	if forfeited == nil || opponent == nil {
		return derror.ErrParticipantNotFound
	}

	if forfeited.IsCompleted() {
		return derror.ErrParticipantFinished
	}

	forfeited.Forfeit()
	opponent.SetResult(valueobject.MatchResultWin)

	winner := opponent.UserID()
	m.winnerID = &winner
	m.isDraw = false
	m.status = valueobject.MatchStatusForfeited
	ended := time.Now().UTC()
	m.endedAt = &ended

	return nil
}

// Timeout xử lý khi trận đấu quá hạn thời gian
func (m *SoloMatch) Timeout(now time.Time) error {
	if m.status != valueobject.MatchStatusInProgress {
		return derror.ErrMatchNotActive
	}

	if now.Before(m.timeoutAt) {
		return derror.ErrMatchNotTimedOutYet
	}

	ended := now.UTC()
	m.endedAt = &ended

	p1 := m.participants[0]
	p2 := m.participants[1]

	if !p1.IsCompleted() && !p2.IsCompleted() {
		m.status = valueobject.MatchStatusCancelled
		p1.SetResult(valueobject.MatchResultForfeited)
		p2.SetResult(valueobject.MatchResultForfeited)
		return nil
	}

	m.status = valueobject.MatchStatusForfeited
	m.isDraw = false

	if p1.IsCompleted() && !p2.IsCompleted() {
		p1.SetResult(valueobject.MatchResultWin)
		p2.Forfeit()
		winner := p1.UserID()
		m.winnerID = &winner
	} else {
		p2.SetResult(valueobject.MatchResultWin)
		p1.Forfeit()
		winner := p2.UserID()
		m.winnerID = &winner
	}

	return nil
}

// HandleTimeout là alias cho Timeout
func (m *SoloMatch) HandleTimeout(now time.Time) error {
	return m.Timeout(now)
}
