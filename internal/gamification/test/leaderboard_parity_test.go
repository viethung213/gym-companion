package test_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/gamification/test/testutil"
)

func TestLeaderboardParity_SQLite(t *testing.T) {
	db, _ := testutil.NewTestDB(t)
	ctx := context.Background()
	seeds := parityDataset()
	seedParityData(ctx, t, db, seeds)

	redisRepo := buildRedisLeaderboard(ctx, t, db)
	assertLeaderboardParity(ctx, t, persistence.NewGormLeaderboardRepository(db), redisRepo, seedIDs(seeds))
}
