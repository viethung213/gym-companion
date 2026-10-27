package persistence

import (
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

// SoloMatchModel ánh xạ bảng solo_match.solo_matches
type SoloMatchModel struct {
	ID            string             `gorm:"primaryKey;column:id"`
	ExerciseID    string             `gorm:"column:exercise_id;not null"`
	ExerciseName  string             `gorm:"column:exercise_name;not null"`
	ChallengeType string             `gorm:"column:challenge_type;not null"`
	Status        string             `gorm:"column:status;not null"`
	WinnerID      *string            `gorm:"column:winner_id"`
	IsDraw        bool               `gorm:"column:is_draw;default:false"`
	StartedAt     *time.Time         `gorm:"column:started_at"`
	ReadyDeadline time.Time          `gorm:"column:ready_deadline;not null"`
	EndedAt       *time.Time         `gorm:"column:ended_at"`
	CreatedAt     time.Time          `gorm:"column:created_at"`
	UpdatedAt     time.Time          `gorm:"column:updated_at"`
	Participants  []ParticipantModel `gorm:"foreignKey:MatchID;constraint:OnDelete:CASCADE"`
}

func (SoloMatchModel) TableName() string {
	return "solo_match.solo_matches"
}

func (m *SoloMatchModel) ToDomain() (*aggregate.SoloMatch, error) {
	cType, err := valueobject.NewChallengeType(m.ChallengeType)
	if err != nil {
		return nil, fmt.Errorf("invalid challenge type %q: %w", m.ChallengeType, err)
	}

	mStatus, err := valueobject.NewMatchStatus(m.Status)
	if err != nil {
		return nil, fmt.Errorf("invalid match status %q: %w", m.Status, err)
	}

	participants := make([]*entity.MatchParticipant, 0, len(m.Participants))
	for i := range m.Participants {
		pDomain, err := m.Participants[i].ToDomain()
		if err != nil {
			return nil, fmt.Errorf("convert participant %s: %w", m.Participants[i].ID, err)
		}
		participants = append(participants, pDomain)
	}

	return aggregate.Reconstitute(
		m.ID,
		m.ExerciseID,
		m.ExerciseName,
		cType,
		mStatus,
		participants,
		m.WinnerID,
		m.IsDraw,
		m.StartedAt,
		m.ReadyDeadline,
		m.EndedAt,
	), nil
}

func FromDomainSoloMatch(m *aggregate.SoloMatch) *SoloMatchModel {
	participants := make([]ParticipantModel, 0, len(m.Participants()))
	for _, p := range m.Participants() {
		participants = append(participants, *FromDomainParticipant(p))
	}

	return &SoloMatchModel{
		ID:            m.ID(),
		ExerciseID:    m.ExerciseID(),
		ExerciseName:  m.ExerciseName(),
		ChallengeType: m.ChallengeType().String(),
		Status:        m.Status().String(),
		WinnerID:      m.WinnerID(),
		IsDraw:        m.IsDraw(),
		StartedAt:     m.StartedAt(),
		ReadyDeadline: m.ReadyDeadline(),
		EndedAt:       m.EndedAt(),
		Participants:  participants,
	}
}

// ParticipantModel ánh xạ bảng solo_match.match_participants
type ParticipantModel struct {
	ID          string     `gorm:"primaryKey;column:id"`
	MatchID     string     `gorm:"column:match_id;not null"`
	UserID      string     `gorm:"column:user_id;not null"`
	ScoreValue  float64    `gorm:"column:score_value;default:0"`
	IsCompleted bool       `gorm:"column:is_completed;default:false"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	Result      string     `gorm:"column:result;default:'PENDING'"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (ParticipantModel) TableName() string {
	return "solo_match.match_participants"
}

func (p *ParticipantModel) ToDomain() (*entity.MatchParticipant, error) {
	score, err := valueobject.NewParticipantScore(p.ScoreValue)
	if err != nil {
		return nil, fmt.Errorf("invalid participant score %v: %w", p.ScoreValue, err)
	}

	res, err := valueobject.NewMatchResult(p.Result)
	if err != nil {
		return nil, fmt.Errorf("invalid match result %q: %w", p.Result, err)
	}

	return entity.ReconstituteParticipant(
		p.ID,
		p.MatchID,
		p.UserID,
		score,
		p.IsCompleted,
		p.CompletedAt,
		res,
	), nil
}

func FromDomainParticipant(p *entity.MatchParticipant) *ParticipantModel {
	return &ParticipantModel{
		ID:          p.ID(),
		MatchID:     p.MatchID(),
		UserID:      p.UserID(),
		ScoreValue:  p.Score().Value(),
		IsCompleted: p.IsCompleted(),
		CompletedAt: p.CompletedAt(),
		Result:      p.Result().String(),
	}
}

// UserSnapshotModel ánh xạ bảng solo_match.user_snapshots
type UserSnapshotModel struct {
	UserID      string    `gorm:"primaryKey;column:user_id"`
	DisplayName string    `gorm:"column:display_name;not null"`
	AvatarURL   string    `gorm:"column:avatar_url;default:''"`
	SyncedAt    time.Time `gorm:"column:synced_at;not null"`
}

func (UserSnapshotModel) TableName() string {
	return "solo_match.user_snapshots"
}

func (u *UserSnapshotModel) ToDomain() (*entity.UserSnapshot, error) {
	return entity.NewUserSnapshot(u.UserID, u.DisplayName, u.AvatarURL, u.SyncedAt)
}

func FromDomainUserSnapshot(s *entity.UserSnapshot) *UserSnapshotModel {
	return &UserSnapshotModel{
		UserID:      s.UserID(),
		DisplayName: s.DisplayName(),
		AvatarURL:   s.AvatarURL(),
		SyncedAt:    s.SyncedAt(),
	}
}

// OutboxModel ánh xạ bảng solo_match.outbox (Transactional Outbox Pattern với FOR UPDATE SKIP LOCKED)
type OutboxModel struct {
	ID           string     `gorm:"primaryKey;column:id"`
	EventID      string     `gorm:"column:event_id;not null"`
	EventType    string     `gorm:"column:event_type;not null"`
	Payload      []byte     `gorm:"column:payload;type:jsonb;not null"`
	PartitionKey string     `gorm:"column:partition_key;not null"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	Published    bool       `gorm:"column:published;default:false"`
	PublishedAt  *time.Time `gorm:"column:published_at"`
	Status       string     `gorm:"column:status;default:'PENDING';not null"`
	LockedUntil  *time.Time `gorm:"column:locked_until"`
}

func (OutboxModel) TableName() string {
	return "solo_match.outbox"
}

// OutboxLogModel ánh xạ bảng solo_match.outbox_log (Idempotency Pattern cho Incoming Consumer Events)
type OutboxLogModel struct {
	ID           string    `gorm:"primaryKey;column:id"`
	EventID      string    `gorm:"column:event_id;not null"`
	EventType    string    `gorm:"column:event_type;not null"`
	Payload      []byte    `gorm:"column:payload;type:jsonb;not null"`
	PartitionKey string    `gorm:"column:partition_key;not null"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
	Status       string    `gorm:"column:status;not null"`
	ErrorMessage string    `gorm:"column:error_message"`
}

func (OutboxLogModel) TableName() string {
	return "solo_match.outbox_log"
}

// SystemBotModel ánh xạ bảng solo_match.system_bots
type SystemBotModel struct {
	UserID          string             `gorm:"primaryKey;column:user_id"`
	DifficultyLevel string             `gorm:"column:difficulty_level;default:'INTERMEDIATE'"`
	IsActive        bool               `gorm:"column:is_active;default:true"`
	CreatedAt       time.Time          `gorm:"column:created_at"`
	UserSnapshot    *UserSnapshotModel `gorm:"foreignKey:UserID;references:UserID"`
}

func (SystemBotModel) TableName() string {
	return "solo_match.system_bots"
}
