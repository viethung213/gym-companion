//go:build integration

package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
)

// plannedBlock is an empty roadmap held as PLANNED from start.
func plannedBlock(t *testing.T, userID string, start time.Time) *roadmap.Roadmap {
	t.Helper()
	rm, err := roadmap.NewRoadmap(&roadmap.Info{
		RoadmapID: uuid.NewString(), UserID: userID, Timezone: "Asia/Ho_Chi_Minh",
		StartDate: start, EndDate: start.AddDate(0, 0, 28),
	}, nil, start)
	require.NoError(t, err)
	require.NoError(t, rm.HoldUntilStart())
	return rm
}

// The next block is stored as PLANNED next to the ACTIVE one, one per user,
// kept out of the history list, and found by the lifecycle queries.
func TestRoadmapRepository_Rotation_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	repo := persistence.NewRoadmapRepository(db)
	ctx := context.Background()
	userID := uuid.NewString()
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.roadmaps WHERE user_id = ?", userID) })

	active := newPersistedRoadmap(t, userID) // 2026-11-23 .. 2026-12-21
	require.NoError(t, repo.Save(ctx, active))
	mine := port.RoadmapRef{RoadmapID: active.ID(), UserID: userID}
	refs, err := repo.ActiveRoadmapsEndingBy(ctx, time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Contains(t, refs, mine, "listed once the bound reaches its end date")
	refs, err = repo.ActiveRoadmapsEndingBy(ctx, time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NotContains(t, refs, mine, "not before")

	next := plannedBlock(t, userID, time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC))
	require.NoError(t, repo.Save(ctx, next))
	require.ErrorIs(t, repo.Save(ctx, plannedBlock(t, userID, time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC))),
		roadmap.ErrPlannedRoadmapExists)

	got, err := repo.FindPlannedByUser(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, next.ID(), got.ID())
	require.Equal(t, roadmap.StatusPlanned, got.Status())

	history, err := repo.ListByUser(ctx, userID, []roadmap.Status{roadmap.StatusActive, roadmap.StatusCompleted}, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1, "the planned block is not part of the history")

	// A roadmap activated in the meantime is never deleted as a plan.
	require.NoError(t, repo.DeletePlannedRoadmap(ctx, active.ID()))
	still, err := repo.FindActiveByUser(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, active.ID(), still.ID())

	require.NoError(t, repo.DeletePlannedRoadmap(ctx, next.ID()))
	_, err = repo.FindPlannedByUser(ctx, userID)
	require.ErrorIs(t, err, roadmap.ErrRoadmapNotFound)
}
