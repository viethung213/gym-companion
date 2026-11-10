package port

import (
	"context"
)

// ParticipantProgress represents live progress of a player in a solo match.
type ParticipantProgress struct {
	UserID         string  `json:"user_id"`
	Value          int32   `json:"value"`
	FormScore      float32 `json:"form_score"`
	Status         string  `json:"status"`
	DisconnectedAt int64   `json:"disconnected_at,omitempty"`
}

// ActiveMatchConfig holds the match parameters for an active user session allowing reconnect by userID.
type ActiveMatchConfig struct {
	MatchID            string `json:"match_id"`
	PlanID             string `json:"plan_id"`
	ExerciseID         string `json:"exercise_id"`
	ChallengeType      string `json:"challenge_type"`
	TimeCeilingSeconds int32  `json:"time_ceiling_seconds"`
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

	// GetMyProgress fetches the recorded progress of the specified user in the match room.
	GetMyProgress(ctx context.Context, matchID, userID string) (*ParticipantProgress, error)

	// GetOpponentProgress fetches the latest progress of the single opponent in the 1v1 match.
	GetOpponentProgress(ctx context.Context, matchID, myUserID string) (*ParticipantProgress, error)

	// SubscribeMatchUpdates subscribes to updates from the opponent in the match.
	SubscribeMatchUpdates(ctx context.Context, matchID, myUserID string) (<-chan ParticipantProgress, func(), error)

	// DelRoom explicitly purges all Redis room keys for a cancelled or ended match.
	DelRoom(ctx context.Context, matchID string) error

	// SetActiveUserMatch stores the active match config pointer for a user allowing reconnect by userID.
	SetActiveUserMatch(ctx context.Context, userID string, cfg ActiveMatchConfig) error

	// GetActiveUserMatch retrieves the active match config pointer for a user.
	GetActiveUserMatch(ctx context.Context, userID string) (*ActiveMatchConfig, error)

	// ClearActiveUserMatch removes the active match config pointer for a user.
	ClearActiveUserMatch(ctx context.Context, userID string) error
}
