//go:build integration

package persistence_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
)

// An inbound event is claimed until it is processed once: a processed one stays
// a duplicate, a failed one can be retried, and an unfinished claim (the
// process died mid-handling) is taken over by the redelivery.
func TestOutboxRepository_EventClaims_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	repo := persistence.NewOutboxRepository(db)
	ctx := context.Background()
	claim := func(id string) bool {
		t.Helper()
		ok, err := repo.ClaimEvent(ctx, id, "test.event", []byte(`{}`))
		require.NoError(t, err)
		return ok
	}

	done := uuid.NewString()
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.outbox_log WHERE event_id = ?", done) })
	require.True(t, claim(done), "a new event is claimed")
	require.NoError(t, repo.CompleteEvent(ctx, done))
	require.False(t, claim(done), "a processed event stays a duplicate")

	failed := uuid.NewString()
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.outbox_log WHERE event_id = ?", failed) })
	require.True(t, claim(failed))
	require.NoError(t, repo.ReleaseEvent(ctx, failed))
	require.True(t, claim(failed), "a released event is claimed again")

	crashed := uuid.NewString()
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.outbox_log WHERE event_id = ?", crashed) })
	require.True(t, claim(crashed))
	// No CompleteEvent: the process died. The redelivery takes the claim over.
	require.True(t, claim(crashed), "an unfinished claim is taken over by the redelivery")
	require.NoError(t, repo.CompleteEvent(ctx, crashed))
	require.False(t, claim(crashed), "once processed it is a duplicate")
}
