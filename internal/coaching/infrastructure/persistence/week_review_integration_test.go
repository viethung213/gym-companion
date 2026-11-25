//go:build integration

package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
)

// A week's review stamp survives a save and load; reviewed weeks leave the queue.
func TestRoadmapRepository_WeekReview_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	repo := persistence.NewRoadmapRepository(db)
	ctx := context.Background()
	userID := uuid.NewString()
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.roadmaps WHERE user_id = ?", userID) })

	rm := newPersistedRoadmap(t, userID)
	require.NoError(t, repo.Save(ctx, rm))
	weekOneEnded := time.Date(2026, 11, 30, 12, 0, 0, 0, time.UTC)

	ids, err := repo.RoadmapIDsWithOpenWeekEndingBy(ctx, weekOneEnded)
	require.NoError(t, err)
	require.Contains(t, ids, rm.ID())

	weekOne := rm.Weeks()[0]
	require.NoError(t, rm.SkipMissedSessions(weekOne.ID()))
	require.NoError(t, rm.MarkReviewed(weekOne.ID(), weekOneEnded))
	require.NoError(t, repo.Save(ctx, rm))

	loaded, err := repo.FindByID(ctx, rm.ID())
	require.NoError(t, err)
	require.True(t, loaded.Weeks()[0].Reviewed())
	require.False(t, loaded.Weeks()[1].Reviewed())

	ids, err = repo.RoadmapIDsWithOpenWeekEndingBy(ctx, weekOneEnded)
	require.NoError(t, err)
	require.NotContains(t, ids, rm.ID())
}
