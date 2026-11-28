package test_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
	"github.com/viethung213/gym-companion/internal/gamification/test/testutil"
)

// A newly registered user must appear on the Redis leaderboard right after initialization, with the
// real repositories: the PostgreSQL repository seeds the 0-XP row inside FindByIDForUpdate, which a
// mock that returns nil does not reproduce.
func TestInitUser_NewUserImmediatelyOnRedisLeaderboard(t *testing.T) {
	ctx := context.Background()
	db, repos := testutil.NewTestDB(t)
	client := goredis.NewClient(&goredis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = client.Close() })
	board := buildRedisLeaderboardWithClient(ctx, t, db, client)

	const userID = "0b6c1f3e-0000-4000-8000-0000000000aa"
	projection := lbredis.NewLeaderboardProjection(client, "init-test")
	handler := command.NewInitUserGamificationHandler(
		repos.UserXpRepo, repos.UserStreakRepo, repos.UserSnapshotRepo, repos.TxManager,
	).WithLeaderboardProjector(command.NewProjectLeaderboardScoreHandler(projection))
	if err := handler.Handle(ctx, command.InitUserGamificationCommand{
		UserID: userID, DisplayName: "Newbie",
	}); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	top, total, err := board.GetGlobalTop(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetGlobalTop: %v", err)
	}
	if total != 1 || len(top) != 1 || top[0].UserID != userID || top[0].Xp != 0 ||
		top[0].DisplayName != "Newbie" {
		t.Fatalf("redis top = %+v (total %d), want the new user with 0 XP named Newbie", top, total)
	}
}
