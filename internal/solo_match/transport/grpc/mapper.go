package grpc

import (
	solomatchv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/message"
	"github.com/viethung213/gym-companion/internal/solo_match/application/dto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ToSoloMatchView(d *dto.SoloMatchDTO) *solomatchv1message.SoloMatchView {
	if d == nil {
		return nil
	}

	participants := make([]*solomatchv1message.ParticipantView, 0, len(d.Participants))
	for _, p := range d.Participants {
		participants = append(participants, &solomatchv1message.ParticipantView{
			User: &solomatchv1message.UserSnapshotView{
				UserId:      p.User.UserID,
				DisplayName: p.User.DisplayName,
				AvatarUrl:   p.User.AvatarURL,
			},
			ScoreValue:  float32(p.ScoreValue),
			IsCompleted: p.IsCompleted,
			Result:      p.Result,
		})
	}

	var startedAt *timestamppb.Timestamp
	if d.StartedAt != nil {
		startedAt = timestamppb.New(*d.StartedAt)
	}

	var endedAt *timestamppb.Timestamp
	if d.EndedAt != nil {
		endedAt = timestamppb.New(*d.EndedAt)
	}

	return &solomatchv1message.SoloMatchView{
		MatchId:       d.MatchID,
		ExerciseId:    d.ExerciseID,
		ExerciseName:  d.ExerciseName,
		ChallengeType: d.ChallengeType,
		Status:        d.Status,
		Participants:  participants,
		WinnerId:      d.WinnerID,
		IsDraw:        d.IsDraw,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
	}
}
