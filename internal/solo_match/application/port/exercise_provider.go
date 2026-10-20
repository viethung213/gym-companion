package port

import (
	"context"
	"time"

	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

// ExerciseChallenge contains competitive exercise information for a solo match.
type ExerciseChallenge struct {
	ExerciseID    string
	ExerciseName  string
	ChallengeType valueobject.ChallengeType
	Duration      time.Duration
}

// ExerciseProvider defines the outbound port interface to fetch competitive exercise challenges
// from an external source or Exercise module/API.
type ExerciseProvider interface {
	GetRandomChallenge(ctx context.Context) (*ExerciseChallenge, error)
}
