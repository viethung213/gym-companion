package dto

import (
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
)

type UserSnapshotDTO struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}

type ParticipantDTO struct {
	User        UserSnapshotDTO `json:"user"`
	IsBot       bool            `json:"isBot"`
	ScoreValue  float64         `json:"scoreValue"`
	IsCompleted bool            `json:"isCompleted"`
	Result      string          `json:"result"`
}

type SoloMatchDTO struct {
	MatchID       string           `json:"matchId"`
	ExerciseID    string           `json:"exerciseId"`
	ExerciseName  string           `json:"exerciseName"`
	ChallengeType string           `json:"challengeType"`
	Status        string           `json:"status"`
	Participants  []ParticipantDTO `json:"participants"`
	WinnerID      string           `json:"winnerId,omitempty"`
	IsDraw        bool             `json:"isDraw"`
	StartedAt     time.Time        `json:"startedAt"`
	TimeoutAt     time.Time        `json:"timeoutAt"`
	EndedAt       *time.Time       `json:"endedAt,omitempty"`
}

func ToSoloMatchDTO(m *aggregate.SoloMatch, snapshots map[string]*entity.UserSnapshot, botFlags ...map[string]bool) SoloMatchDTO {
	var botMap map[string]bool
	if len(botFlags) > 0 {
		botMap = botFlags[0]
	}

	var participants []ParticipantDTO
	for _, p := range m.Participants() {
		snap := snapshots[p.UserID()]
		snapDTO := UserSnapshotDTO{
			UserID:      p.UserID(),
			DisplayName: "Gym Athlete",
		}
		if snap != nil {
			snapDTO.DisplayName = snap.DisplayName()
			snapDTO.AvatarURL = snap.AvatarURL()
		}

		isBot := false
		if botMap != nil {
			isBot = botMap[p.UserID()]
		}

		participants = append(participants, ParticipantDTO{
			User:        snapDTO,
			IsBot:       isBot,
			ScoreValue:  p.Score().Value(),
			IsCompleted: p.IsCompleted(),
			Result:      p.Result().String(),
		})
	}

	winner := ""
	if m.WinnerID() != nil {
		winner = *m.WinnerID()
	}

	return SoloMatchDTO{
		MatchID:       m.ID(),
		ExerciseID:    m.ExerciseID(),
		ExerciseName:  m.ExerciseName(),
		ChallengeType: m.ChallengeType().String(),
		Status:        m.Status().String(),
		Participants:  participants,
		WinnerID:      winner,
		IsDraw:        m.IsDraw(),
		StartedAt:     m.StartedAt(),
		TimeoutAt:     m.TimeoutAt(),
		EndedAt:       m.EndedAt(),
	}
}
