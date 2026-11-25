//go:build integration

package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
)

// A writer that read the roadmap before someone else saved it (a regenerate
// waiting on the model while a workout completes) is refused instead of
// writing its stale copy over the completion.
func TestRoadmapRepository_StaleSaveIsRefused_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	repo := persistence.NewRoadmapRepository(db)
	ctx := context.Background()
	userID := uuid.NewString()
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.roadmaps WHERE user_id = ?", userID) })

	rm := newPersistedRoadmap(t, userID)
	require.NoError(t, repo.Save(ctx, rm))

	slow, err := repo.FindByID(ctx, rm.ID()) // e.g. a regenerate starts
	require.NoError(t, err)
	fast, err := repo.FindByID(ctx, rm.ID()) // e.g. a completion lands
	require.NoError(t, err)

	pending := fast.Weeks()[0].Days()[1].Sessions()[0]
	require.NoError(t, pending.MarkCompleted(100, 0, time.Now()))
	require.NoError(t, repo.Save(ctx, fast))

	require.ErrorIs(t, repo.Save(ctx, slow), roadmap.ErrRoadmapChanged)

	stored, err := repo.FindByID(ctx, rm.ID())
	require.NoError(t, err)
	got, _ := stored.FindSession(pending.ID())
	require.Equal(t, roadmap.SessionPlanStatusCompleted, got.Status(), "the completion survives")

	require.NoError(t, repo.Save(ctx, stored), "a fresh copy saves fine")
}
