package redis_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/redis"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *goredis.Client) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	return mr, client
}

func TestRedisCronTaskCoordinator_EnsureTaskSet_WinnerLoadsAndOthersWait(t *testing.T) {
	t.Parallel()
	_, client := setupTestRedis(t)
	defer client.Close()

	ctx := context.Background()
	coord1 := redis.NewCronTaskCoordinator(client).WithPollSettings(10*time.Millisecond, 2*time.Second)
	coord2 := redis.NewCronTaskCoordinator(client).WithPollSettings(10*time.Millisecond, 2*time.Second)

	loaderCalls := 0
	var mu sync.Mutex
	loaderFn := func() ([]string, error) {
		mu.Lock()
		loaderCalls++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond) // Simulate DB query latency
		return []string{"user-1", "user-2", "user-3"}, nil
	}

	var wg sync.WaitGroup
	var res1, res2 []string
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		res1, err1 = coord1.EnsureTaskSet(ctx, "weekly_insight", "2026-11-15", loaderFn)
	}()
	go func() {
		defer wg.Done()
		res2, err2 = coord2.EnsureTaskSet(ctx, "weekly_insight", "2026-11-15", loaderFn)
	}()

	wg.Wait()

	if err1 != nil {
		t.Fatalf("coord1 error: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("coord2 error: %v", err2)
	}

	if loaderCalls != 1 {
		t.Fatalf("expected exactly 1 DB loader call, got %d", loaderCalls)
	}

	if len(res1) != 3 || len(res2) != 3 {
		t.Fatalf("expected both to get 3 users, got res1=%d, res2=%d", len(res1), len(res2))
	}
}

func TestRedisCronTaskCoordinator_EnsureTaskSet_EmptyList(t *testing.T) {
	t.Parallel()
	_, client := setupTestRedis(t)
	defer client.Close()

	ctx := context.Background()
	coord1 := redis.NewCronTaskCoordinator(client)
	coord2 := redis.NewCronTaskCoordinator(client)

	loaderCalls := 0
	loaderFn := func() ([]string, error) {
		loaderCalls++
		return []string{}, nil
	}

	res1, err := coord1.EnsureTaskSet(ctx, "daily_menu", "2026-11-16", loaderFn)
	if err != nil {
		t.Fatalf("coord1 error: %v", err)
	}
	if len(res1) != 0 {
		t.Fatalf("expected empty list, got %v", res1)
	}

	// Second coordinator should see empty marker and not call loaderFn
	res2, err := coord2.EnsureTaskSet(ctx, "daily_menu", "2026-11-16", loaderFn)
	if err != nil {
		t.Fatalf("coord2 error: %v", err)
	}
	if len(res2) != 0 {
		t.Fatalf("expected empty list, got %v", res2)
	}
	if loaderCalls != 1 {
		t.Fatalf("expected 1 loader call, got %d", loaderCalls)
	}
}

func TestRedisCronTaskCoordinator_EnsureTaskSet_LoaderError_ReleasesInitLock(t *testing.T) {
	t.Parallel()
	_, client := setupTestRedis(t)
	defer client.Close()

	ctx := context.Background()
	coord := redis.NewCronTaskCoordinator(client)

	expectedErr := errors.New("db connection failure")
	failingLoader := func() ([]string, error) {
		return nil, expectedErr
	}

	_, err := coord.EnsureTaskSet(ctx, "daily_menu", "2026-11-17", failingLoader)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should be able to try again because initKey was deleted upon failure
	successfulLoader := func() ([]string, error) {
		return []string{"user-retry"}, nil
	}
	res, err := coord.EnsureTaskSet(ctx, "daily_menu", "2026-11-17", successfulLoader)
	if err != nil {
		t.Fatalf("retry error: %v", err)
	}
	if len(res) != 1 || res[0] != "user-retry" {
		t.Fatalf("unexpected res: %v", res)
	}
}

func TestRedisCronTaskCoordinator_TryClaimUser_ContentionAndCompletion(t *testing.T) {
	t.Parallel()
	_, client := setupTestRedis(t)
	defer client.Close()

	ctx := context.Background()
	coordA := redis.NewCronTaskCoordinator(client)
	coordB := redis.NewCronTaskCoordinator(client)

	jobName := "weekly_insight"
	targetDate := "2026-11-15"

	// 1. Initialize tasks
	_, err := coordA.EnsureTaskSet(ctx, jobName, targetDate, func() ([]string, error) {
		return []string{"user-A", "user-B"}, nil
	})
	if err != nil {
		t.Fatalf("ensure task set: %v", err)
	}

	// 2. Node A claims user-A
	claimedA, err := coordA.TryClaimUser(ctx, jobName, targetDate, "user-A", 5*time.Minute)
	if err != nil || !claimedA {
		t.Fatalf("coordA failed to claim user-A: claimed=%v, err=%v", claimedA, err)
	}

	// 3. Node B attempts to claim user-A concurrently -> must fail
	claimedB, err := coordB.TryClaimUser(ctx, jobName, targetDate, "user-A", 5*time.Minute)
	if err != nil {
		t.Fatalf("coordB error: %v", err)
	}
	if claimedB {
		t.Fatal("expected coordB to fail claiming user-A (already locked by Node A)")
	}

	// 4. Node B claims user-B -> must succeed
	claimedBUserB, err := coordB.TryClaimUser(ctx, jobName, targetDate, "user-B", 5*time.Minute)
	if err != nil || !claimedBUserB {
		t.Fatalf("coordB failed to claim user-B: claimed=%v, err=%v", claimedBUserB, err)
	}

	// 5. Node A completes user-A
	if err := coordA.CompleteUser(ctx, jobName, targetDate, "user-A"); err != nil {
		t.Fatalf("complete user-A: %v", err)
	}

	// 6. After completion, claiming user-A must return false (no longer in task set)
	claimedAgain, err := coordA.TryClaimUser(ctx, jobName, targetDate, "user-A", 5*time.Minute)
	if err != nil {
		t.Fatalf("try claim completed user error: %v", err)
	}
	if claimedAgain {
		t.Fatal("expected claim on completed user to return false")
	}

	// 7. Node B releases lock on user-B without completing (simulate transient failure)
	if err := coordB.ReleaseUserLock(ctx, jobName, targetDate, "user-B"); err != nil {
		t.Fatalf("release user-B lock: %v", err)
	}

	// 8. Now user-B can be claimed again
	claimedRetry, err := coordA.TryClaimUser(ctx, jobName, targetDate, "user-B", 5*time.Minute)
	if err != nil || !claimedRetry {
		t.Fatalf("expected user-B to be claimable again: claimed=%v, err=%v", claimedRetry, err)
	}
}
