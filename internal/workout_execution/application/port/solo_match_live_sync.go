package port

import (
	"context"
)

// ParticipantProgress represents live progress of a player in a solo match.
type ParticipantProgress struct {
	UserID    string  `json:"user_id"`
	Value     int32   `json:"value"`
	FormScore float32 `json:"form_score"`
	Status    string  `json:"status"`
}

// SoloMatchLiveSyncPort provides realtime pub/sub synchronization and room membership management between match participants.
type SoloMatchLiveSyncPort interface {
	// RegisterParticipant registers a user in the match room.
	// Returns ErrMatchRoomFull if the room already has 2 different participants.
	// Returns ErrMatchExerciseMismatch if the room has an existing participant with a different exercise.
	RegisterParticipant(ctx context.Context, matchID, userID, exerciseID string) error

	// UnregisterParticipant removes a participant when their stream ends.
	UnregisterParticipant(ctx context.Context, matchID, userID string) error

	// IsMatchReady checks whether both participants have joined the match room.
	IsMatchReady(ctx context.Context, matchID string) (bool, error)

	// ParticipantCount returns the number of participants currently registered in the room.
	ParticipantCount(ctx context.Context, matchID string) (int, error)

	// PublishMyProgress broadcasts participant progress to the room.
	PublishMyProgress(ctx context.Context, matchID string, progress ParticipantProgress) error

	// GetOpponentProgress fetches the latest progress of the single opponent in the 1v1 match.
	GetOpponentProgress(ctx context.Context, matchID, myUserID string) (*ParticipantProgress, error)

	// SubscribeMatchUpdates subscribes to updates from the opponent in the match.
	SubscribeMatchUpdates(ctx context.Context, matchID, myUserID string) (<-chan ParticipantProgress, func(), error)
}
