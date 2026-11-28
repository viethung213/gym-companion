package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/gamification/test/testutil"
	"gorm.io/gorm"
)

func TestGormLeaderboardRepository_Constructor(t *testing.T) {
	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{}}
	repo := persistence.NewGormLeaderboardRepository(dummyDB)
	if repo == nil {
		t.Fatal("NewGormLeaderboardRepository() = nil, want non-nil")
	}
}

func TestUserSnapshotEntity_Constructor(t *testing.T) {
	now := time.Now().UTC()
	snap := entity.NewUserSnapshot("user-1", "Champion Gymer", "https://cdn.example.com/avatar.jpg", now)
	if snap.UserID != "user-1" || snap.DisplayName != "Champion Gymer" || snap.AvatarURL != "https://cdn.example.com/avatar.jpg" {
		t.Errorf("unexpected snapshot entity: %+v", snap)
	}
}

func leaderboardBaseTime() time.Time { return time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC) }

func seedLeaderboardUser(t *testing.T, repos *testutil.Repositories, userID string, xp int64, updatedAt time.Time, displayName string) {
	t.Helper()
	ctx := context.Background()
	if err := repos.UserXpRepo.Save(ctx, aggregate.ReconstituteUserXp(userID, xp, 1, nil, updatedAt)); err != nil {
		t.Fatalf("seed user_xp %s: %v", userID, err)
	}
	if displayName == "" {
		return
	}
	snap := entity.NewUserSnapshot(userID, displayName, "https://cdn/"+userID+".png", updatedAt)
	if err := repos.UserSnapshotRepo.SaveSnapshot(ctx, snap); err != nil {
		t.Fatalf("seed snapshot %s: %v", userID, err)
	}
}

func TestGormLeaderboardRepository_GetGlobalTop_OrderAndDefaults(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	seedLeaderboardUser(t, repos, "u-a", 500, leaderboardBaseTime().Add(2*time.Hour), "Late")
	seedLeaderboardUser(t, repos, "u-b", 500, leaderboardBaseTime(), "Early")
	seedLeaderboardUser(t, repos, "u-d", 300, leaderboardBaseTime(), "Dee")
	seedLeaderboardUser(t, repos, "u-c", 300, leaderboardBaseTime(), "")
	seedLeaderboardUser(t, repos, "u-z", 1000, leaderboardBaseTime(), "Top")

	top, total, err := repos.LeaderboardRepo.GetGlobalTop(ctx, 50, 0)
	if err != nil {
		t.Fatalf("GetGlobalTop: %v", err)
	}

	wantOrder := []string{"u-z", "u-b", "u-a", "u-c", "u-d"}
	if int(total) != len(wantOrder) || len(top) != len(wantOrder) {
		t.Fatalf("got %d rows (total %d), want %d", len(top), total, len(wantOrder))
	}
	for i, id := range wantOrder {
		if top[i].UserID != id || top[i].Rank != int64(i+1) {
			t.Errorf("row %d = (%s, rank %d), want (%s, rank %d)", i, top[i].UserID, top[i].Rank, id, i+1)
		}
	}
	if top[3].DisplayName != "Gymer" || top[3].AvatarURL != "" {
		t.Errorf("missing snapshot defaults = (%q, %q), want (\"Gymer\", \"\")", top[3].DisplayName, top[3].AvatarURL)
	}
	if top[0].DisplayName != "Top" || top[0].AvatarURL != "https://cdn/u-z.png" {
		t.Errorf("snapshot not joined: %+v", top[0])
	}

	page, _, err := repos.LeaderboardRepo.GetGlobalTop(ctx, 2, 1)
	if err != nil {
		t.Fatalf("GetGlobalTop offset: %v", err)
	}
	if len(page) != 2 || page[0].UserID != "u-b" || page[0].Rank != 2 || page[1].UserID != "u-a" || page[1].Rank != 3 {
		t.Errorf("offset page = %+v, want u-b(2), u-a(3)", page)
	}
}

func TestGormLeaderboardRepository_GetUserRank_TieBreak(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	seedLeaderboardUser(t, repos, "u-a", 500, leaderboardBaseTime().Add(time.Hour), "A")
	seedLeaderboardUser(t, repos, "u-b", 500, leaderboardBaseTime(), "B")
	seedLeaderboardUser(t, repos, "u-c", 500, leaderboardBaseTime().Add(time.Hour), "C")

	cases := map[string]int64{"u-b": 1, "u-a": 2, "u-c": 3}
	for userID, wantRank := range cases {
		rank, entry, err := repos.LeaderboardRepo.GetUserRank(ctx, userID)
		if err != nil {
			t.Fatalf("GetUserRank(%s): %v", userID, err)
		}
		if rank != wantRank || entry == nil || entry.Rank != wantRank {
			t.Errorf("GetUserRank(%s) = %d, want %d", userID, rank, wantRank)
		}
	}

	rank, entry, err := repos.LeaderboardRepo.GetUserRank(ctx, "unknown")
	if err != nil || rank != 0 || entry != nil {
		t.Errorf("unknown user = (%d, %+v, %v), want (0, nil, nil)", rank, entry, err)
	}
}

func TestGormLeaderboardRepository_ListScores_KeysetPaging(t *testing.T) {
	db, repos := testutil.NewTestDB(t)
	ctx := context.Background()
	source := persistence.NewGormLeaderboardRepository(db)

	for i, id := range []string{"u-5", "u-1", "u-3", "u-2", "u-4"} {
		seedLeaderboardUser(t, repos, id, int64(100*(i+1)), leaderboardBaseTime().Add(time.Duration(i)*time.Minute), "")
	}

	var got []string
	cursor := ""
	pageSizes := []int{}
	for range 4 {
		page, err := source.ListScores(ctx, cursor, 2)
		if err != nil {
			t.Fatalf("ListScores(%q): %v", cursor, err)
		}
		pageSizes = append(pageSizes, len(page))
		for _, s := range page {
			got = append(got, s.UserID)
		}
		if len(page) > 0 {
			cursor = page[len(page)-1].UserID
		}
	}

	want := []string{"u-1", "u-2", "u-3", "u-4", "u-5"}
	if len(got) != len(want) {
		t.Fatalf("ListScores ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ListScores ids = %v, want %v", got, want)
		}
	}
	if pageSizes[0] != 2 || pageSizes[1] != 2 || pageSizes[2] != 1 || pageSizes[3] != 0 {
		t.Errorf("page sizes = %v, want [2 2 1 0]", pageSizes)
	}

	first, err := source.ListScores(ctx, "", 1)
	if err != nil || len(first) != 1 {
		t.Fatalf("ListScores first: %v %v", first, err)
	}
	if first[0].Xp != 200 || !first[0].UpdatedAt.Equal(leaderboardBaseTime().Add(time.Minute)) {
		t.Errorf("score u-1 = %+v, want xp 200 at %v", first[0], leaderboardBaseTime().Add(time.Minute))
	}
}

func TestGormLeaderboardRepository_GetScore(t *testing.T) {
	db, repos := testutil.NewTestDB(t)
	ctx := context.Background()
	source := persistence.NewGormLeaderboardRepository(db)

	seedLeaderboardUser(t, repos, "u-1", 750, leaderboardBaseTime(), "")

	score, err := source.GetScore(ctx, "u-1")
	if err != nil || score == nil {
		t.Fatalf("GetScore(u-1) = %+v, %v", score, err)
	}
	if score.UserID != "u-1" || score.Xp != 750 || !score.UpdatedAt.Equal(leaderboardBaseTime()) {
		t.Errorf("GetScore(u-1) = %+v", score)
	}

	missing, err := source.GetScore(ctx, "nobody")
	if err != nil || missing != nil {
		t.Errorf("GetScore(nobody) = %+v, %v; want nil, nil", missing, err)
	}
}
