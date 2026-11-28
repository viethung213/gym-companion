package redis_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
)

type stubLeaderboardRepo struct {
	name  string
	err   error
	calls int
}

func (s *stubLeaderboardRepo) GetGlobalTop(context.Context, int, int) ([]*entity.LeaderboardUser, int64, error) {
	s.calls++
	if s.err != nil {
		return nil, 0, s.err
	}
	return []*entity.LeaderboardUser{{Rank: 1, UserID: s.name}}, 1, nil
}

func (s *stubLeaderboardRepo) GetUserRank(context.Context, string) (int64, *entity.LeaderboardUser, error) {
	s.calls++
	if s.err != nil {
		return 0, nil, s.err
	}
	return 7, &entity.LeaderboardUser{Rank: 7, UserID: s.name}, nil
}

func TestFallback_PrimaryError_UsesFallback(t *testing.T) {
	primary := &stubLeaderboardRepo{name: "redis", err: errors.New("connection refused")}
	fallback := &stubLeaderboardRepo{name: "postgres"}
	repo := lbredis.NewFallbackLeaderboardRepository(primary, fallback, nil, time.Minute)
	ctx := context.Background()

	top, _, err := repo.GetGlobalTop(ctx, 50, 0)
	if err != nil || len(top) != 1 || top[0].UserID != "postgres" {
		t.Fatalf("GetGlobalTop = %+v, %v; want postgres result", top, err)
	}
	rank, entry, err := repo.GetUserRank(ctx, "u-1")
	if err != nil || rank != 7 || entry.UserID != "postgres" {
		t.Fatalf("GetUserRank = %d, %+v, %v; want postgres result", rank, entry, err)
	}
}

func TestFallback_NotReady_CallsOnNotReady(t *testing.T) {
	primary := &stubLeaderboardRepo{name: "redis", err: fmt.Errorf("wrap: %w", lbredis.ErrLeaderboardNotReady)}
	fallback := &stubLeaderboardRepo{name: "postgres"}
	signals := 0
	repo := lbredis.NewFallbackLeaderboardRepository(primary, fallback, func() { signals++ }, time.Minute)

	_, _, _ = repo.GetGlobalTop(context.Background(), 50, 0)
	_, _, _ = repo.GetUserRank(context.Background(), "u-1")
	if signals != 2 || fallback.calls != 2 {
		t.Fatalf("signals = %d, fallback calls = %d; want 2, 2", signals, fallback.calls)
	}
}

func TestFallback_OtherError_DoesNotCallOnNotReady(t *testing.T) {
	primary := &stubLeaderboardRepo{name: "redis", err: errors.New("timeout")}
	signals := 0
	repo := lbredis.NewFallbackLeaderboardRepository(
		primary, &stubLeaderboardRepo{name: "postgres"}, func() { signals++ }, time.Minute)

	_, _, _ = repo.GetGlobalTop(context.Background(), 50, 0)
	if signals != 0 {
		t.Fatalf("onNotReady called %d times for a non-readiness error", signals)
	}
}

func TestFallback_PrimaryOK_NoFallback(t *testing.T) {
	primary := &stubLeaderboardRepo{name: "redis"}
	fallback := &stubLeaderboardRepo{name: "postgres"}
	repo := lbredis.NewFallbackLeaderboardRepository(primary, fallback, nil, time.Minute)

	top, _, err := repo.GetGlobalTop(context.Background(), 50, 0)
	if err != nil || top[0].UserID != "redis" || fallback.calls != 0 {
		t.Fatalf("GetGlobalTop = %+v, %v (fallback calls %d); want redis result only", top, err, fallback.calls)
	}
}

func TestFallback_BothFail_ReturnsFallbackError(t *testing.T) {
	repo := lbredis.NewFallbackLeaderboardRepository(
		&stubLeaderboardRepo{err: errors.New("redis down")},
		&stubLeaderboardRepo{err: errors.New("postgres down")},
		nil,
		time.Minute,
	)
	if _, _, err := repo.GetGlobalTop(context.Background(), 50, 0); err == nil {
		t.Fatal("GetGlobalTop err = nil, want error when both sources fail")
	}
}

func TestFallback_PrimaryError_SkipsPrimaryDuringCooldown(t *testing.T) {
	primary := &stubLeaderboardRepo{name: "redis", err: fmt.Errorf("dial: %w", lbredis.ErrRedisUnavailable)}
	fallback := &stubLeaderboardRepo{name: "postgres"}
	repo := lbredis.NewFallbackLeaderboardRepository(primary, fallback, nil, time.Hour)
	ctx := context.Background()

	_, _, _ = repo.GetGlobalTop(ctx, 50, 0)
	_, _, _ = repo.GetUserRank(ctx, "u-1")
	_, _, _ = repo.GetGlobalTop(ctx, 50, 0)

	if primary.calls != 1 {
		t.Fatalf("primary calls = %d, want 1 (later calls must skip a failing Redis)", primary.calls)
	}
	if fallback.calls != 3 {
		t.Fatalf("fallback calls = %d, want 3", fallback.calls)
	}
}

func TestFallback_RetriesPrimaryAfterCooldown(t *testing.T) {
	primary := &stubLeaderboardRepo{name: "redis", err: fmt.Errorf("timeout: %w", lbredis.ErrRedisUnavailable)}
	repo := lbredis.NewFallbackLeaderboardRepository(
		primary, &stubLeaderboardRepo{name: "postgres"}, nil, 20*time.Millisecond)
	ctx := context.Background()

	_, _, _ = repo.GetGlobalTop(ctx, 50, 0)
	primary.err = nil
	time.Sleep(40 * time.Millisecond)

	top, _, err := repo.GetGlobalTop(ctx, 50, 0)
	if err != nil || top[0].UserID != "redis" || primary.calls != 2 {
		t.Fatalf("after cooldown: top %+v, err %v, primary calls %d; want redis served again", top, err, primary.calls)
	}
}

func TestFallback_NotReady_DoesNotSkipPrimary(t *testing.T) {
	primary := &stubLeaderboardRepo{name: "redis", err: lbredis.ErrLeaderboardNotReady}
	repo := lbredis.NewFallbackLeaderboardRepository(
		primary, &stubLeaderboardRepo{name: "postgres"}, nil, time.Hour)
	ctx := context.Background()

	_, _, _ = repo.GetGlobalTop(ctx, 50, 0)
	_, _, _ = repo.GetGlobalTop(ctx, 50, 0)
	if primary.calls != 2 {
		t.Fatalf("primary calls = %d, want 2 (a healthy Redis that is rebuilding must be retried)", primary.calls)
	}
}

func TestFallback_OnlyRedisUnavailableOpensBreaker(t *testing.T) {
	tests := []struct {
		name string
		give error
		want int // primary calls after two requests
	}{
		{name: "redis unavailable", give: fmt.Errorf("read: %w", lbredis.ErrRedisUnavailable), want: 1},
		{name: "client cancelled", give: context.Canceled, want: 2},
		{name: "deadline exceeded", give: context.DeadlineExceeded, want: 2},
		{name: "postgres snapshot error", give: errors.New("load snapshots: db down"), want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &stubLeaderboardRepo{name: "redis", err: tt.give}
			repo := lbredis.NewFallbackLeaderboardRepository(
				primary, &stubLeaderboardRepo{name: "postgres"}, nil, time.Hour)

			_, _, _ = repo.GetGlobalTop(context.Background(), 50, 0)
			_, _, _ = repo.GetGlobalTop(context.Background(), 50, 0)

			if got := primary.calls; got != tt.want {
				t.Fatalf("primary calls = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFallback_CancelledCaller_DoesNotOpenBreaker(t *testing.T) {
	primary := &stubLeaderboardRepo{
		name: "redis",
		err:  fmt.Errorf("i/o: %w", lbredis.ErrRedisUnavailable),
	}
	repo := lbredis.NewFallbackLeaderboardRepository(
		primary, &stubLeaderboardRepo{name: "postgres"}, nil, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, _ = repo.GetGlobalTop(ctx, 50, 0)
	_, _, _ = repo.GetGlobalTop(context.Background(), 50, 0)
	if primary.calls != 2 {
		t.Fatalf("primary calls = %d, want 2 (a cancelled caller says nothing about Redis)", primary.calls)
	}
}
