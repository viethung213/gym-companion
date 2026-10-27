package port

import (
	"context"
	"time"
)

// SoloBotManager manages the lifecycle and background execution of virtual bots in solo matches.
type SoloBotManager interface {
	// StartBot initiates virtual bot simulation for a given match.
	StartBot(ctx context.Context, matchID, botUserID, userPlayerID, exerciseID, challengeType string) error

	// CancelBot cancels the running virtual bot goroutine for the specified match.
	CancelBot(matchID string)

	// StopAll cancels all active virtual bot runners and blocks until all goroutines exit cleanly.
	StopAll()

	// SetRunnerDelaysForTesting allows test suites to customize bot timing for fast execution.
	SetRunnerDelaysForTesting(repInterval, durInterval time.Duration)
}
