package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

func setupTestRedis(t *testing.T) (*RedisLiveSyncHub, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	hub := NewRedisLiveSyncHub(client, time.Hour)
	return hub, mr
}

func TestRedisLiveSyncHub_RoomAdmission(t *testing.T) {
	hub, _ := setupTestRedis(t)
	ctx := context.Background()
	matchID := "match-admission-101"

	// 1. Initial count should be 0
	cnt, err := hub.ParticipantCount(ctx, matchID)
	if err != nil {
		t.Fatalf("unexpected error getting participant count: %v", err)
	}
	if cnt != 0 {
		t.Errorf("got %d participants, want 0", cnt)
	}

	ready, err := hub.IsMatchReady(ctx, matchID)
	if err != nil {
		t.Fatalf("unexpected error checking ready: %v", err)
	}
	if ready {
		t.Errorf("got ready true, want false")
	}

	// 2. User 1 joins
	if err := hub.RegisterParticipant(ctx, matchID, "user-1", "plank_01"); err != nil {
		t.Fatalf("failed to register user 1: %v", err)
	}
	cnt, _ = hub.ParticipantCount(ctx, matchID)
	if cnt != 1 {
		t.Errorf("got %d participants, want 1", cnt)
	}
	ready, _ = hub.IsMatchReady(ctx, matchID)
	if ready {
		t.Errorf("got ready true with 1 participant, want false")
	}

	// 3. User 1 re-registers (idempotent)
	if err := hub.RegisterParticipant(ctx, matchID, "user-1", "plank_01"); err != nil {
		t.Fatalf("re-registration should succeed, got: %v", err)
	}
	cnt, _ = hub.ParticipantCount(ctx, matchID)
	if cnt != 1 {
		t.Errorf("got %d participants after re-registration, want 1", cnt)
	}

	// 4. User 2 joins
	if err := hub.RegisterParticipant(ctx, matchID, "user-2", "plank_01"); err != nil {
		t.Fatalf("failed to register user 2: %v", err)
	}
	cnt, _ = hub.ParticipantCount(ctx, matchID)
	if cnt != 2 {
		t.Errorf("got %d participants, want 2", cnt)
	}
	ready, _ = hub.IsMatchReady(ctx, matchID)
	if !ready {
		t.Errorf("got ready false with 2 participants, want true")
	}

	// 5. User 3 joins -> must be rejected with ErrMatchRoomFull
	err = hub.RegisterParticipant(ctx, matchID, "user-3", "plank_01")
	if !errors.Is(err, apperror.ErrMatchRoomFull) {
		t.Errorf("got error %v, want %v", err, apperror.ErrMatchRoomFull)
	}

	// 6. User 1 unregisters -> Room Lock preserves participants set to lock out 3rd party
	if err := hub.UnregisterParticipant(ctx, matchID, "user-1"); err != nil {
		t.Fatalf("failed to unregister user 1: %v", err)
	}
	// ParticipantCount remains 2 because user is not SRem'd
	cnt, _ = hub.ParticipantCount(ctx, matchID)
	if cnt != 2 {
		t.Errorf("got %d participants after unregister, want 2 (room lock)", cnt)
	}
	// User 3 tries to join locked room -> must still be rejected
	err = hub.RegisterParticipant(ctx, matchID, "user-3", "plank_01")
	if !errors.Is(err, apperror.ErrMatchRoomFull) {
		t.Errorf("got error %v, want %v when 3rd party joins locked room", err, apperror.ErrMatchRoomFull)
	}
	// User 1 re-connects (idempotent entry into locked room)
	if err := hub.RegisterParticipant(ctx, matchID, "user-1", "plank_01"); err != nil {
		t.Fatalf("re-registration of user-1 into locked room should succeed, got: %v", err)
	}
}

func TestRedisLiveSyncHub_ExerciseMismatch(t *testing.T) {
	hub, _ := setupTestRedis(t)
	ctx := context.Background()
	matchID := "match-mismatch-exercise"

	// 1. User 1 registers with plank_01
	if err := hub.RegisterParticipant(ctx, matchID, "user-1", "plank_01"); err != nil {
		t.Fatalf("failed to register user-1: %v", err)
	}

	// 2. User 2 registers with a different exercise -> must fail
	err := hub.RegisterParticipant(ctx, matchID, "user-2", "squat_01")
	if !errors.Is(err, apperror.ErrMatchExerciseMismatch) {
		t.Errorf("got error %v, want %v", err, apperror.ErrMatchExerciseMismatch)
	}

	// 3. User 2 registers with the same exercise -> must succeed
	if err := hub.RegisterParticipant(ctx, matchID, "user-2", "plank_01"); err != nil {
		t.Fatalf("failed to register user-2 with matching exercise: %v", err)
	}
}

func TestRedisLiveSyncHub_ProgressAndPubSub(t *testing.T) {
	hub, _ := setupTestRedis(t)
	ctx := context.Background()
	matchID := "match-pubsub-202"

	// Register 2 participants
	if err := hub.RegisterParticipant(ctx, matchID, "user-A", "push_up_01"); err != nil {
		t.Fatalf("register user-A: %v", err)
	}
	if err := hub.RegisterParticipant(ctx, matchID, "user-B", "push_up_01"); err != nil {
		t.Fatalf("register user-B: %v", err)
	}

	// Subscribe User A to match stream
	subCh, unsub, err := hub.SubscribeMatchUpdates(ctx, matchID, "user-A")
	if err != nil {
		t.Fatalf("subscribe user-A: %v", err)
	}
	defer unsub()

	// User B publishes progress
	progB := port.ParticipantProgress{
		UserID:    "user-B",
		Value:     15,
		FormScore: 92.5,
		Status:    "ACTIVE",
	}
	if err := hub.PublishMyProgress(ctx, matchID, progB); err != nil {
		t.Fatalf("publish progress user-B: %v", err)
	}

	// User A publishes own progress (should be ignored by User A's subscription)
	progA := port.ParticipantProgress{
		UserID:    "user-A",
		Value:     10,
		FormScore: 88.0,
		Status:    "ACTIVE",
	}
	if err := hub.PublishMyProgress(ctx, matchID, progA); err != nil {
		t.Fatalf("publish progress user-A: %v", err)
	}

	// User A receives User B's progress
	select {
	case received := <-subCh:
		if received.UserID != "user-B" {
			t.Errorf("got UserID %s, want user-B", received.UserID)
		}
		if received.Value != 15 {
			t.Errorf("got Value %d, want 15", received.Value)
		}
		if received.Status != "ACTIVE" {
			t.Errorf("got Status %s, want ACTIVE", received.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for opponent progress via Redis Pub/Sub")
	}

	// Test GetOpponentProgress for User A
	oppProg, err := hub.GetOpponentProgress(ctx, matchID, "user-A")
	if err != nil {
		t.Fatalf("get opponent progress: %v", err)
	}
	if oppProg == nil {
		t.Fatal("expected opponent progress, got nil")
	}
	if oppProg.UserID != "user-B" {
		t.Errorf("got opponent UserID %s, want user-B", oppProg.UserID)
	}
	if oppProg.Value != 15 {
		t.Errorf("got opponent Value %d, want 15", oppProg.Value)
	}
}

func TestRedisLiveSyncHub_RoomCleanupOnAllFinished(t *testing.T) {
	hub, mr := setupTestRedis(t)
	ctx := context.Background()
	matchID := "match-cleanup-test"

	// 1. Both users join
	if err := hub.RegisterParticipant(ctx, matchID, "user-1", "squat_01"); err != nil {
		t.Fatalf("register user-1: %v", err)
	}
	if err := hub.RegisterParticipant(ctx, matchID, "user-2", "squat_01"); err != nil {
		t.Fatalf("register user-2: %v", err)
	}

	// 2. User 1 completes and unregisters
	prog1 := port.ParticipantProgress{
		UserID:    "user-1",
		Value:     20,
		FormScore: 95.0,
		Status:    "COMPLETED",
	}
	if err := hub.PublishMyProgress(ctx, matchID, prog1); err != nil {
		t.Fatalf("publish user-1 progress: %v", err)
	}
	if err := hub.UnregisterParticipant(ctx, matchID, "user-1"); err != nil {
		t.Fatalf("unregister user-1: %v", err)
	}

	// Room should NOT be cleaned up yet because user-2 is still active
	partKey := "solo_match:" + matchID + ":participants"
	if !mr.Exists(partKey) {
		t.Fatalf("expected participants key to exist while user-2 is active")
	}

	// 3. User 2 completes and unregisters
	prog2 := port.ParticipantProgress{
		UserID:    "user-2",
		Value:     18,
		FormScore: 90.0,
		Status:    "COMPLETED",
	}
	if err := hub.PublishMyProgress(ctx, matchID, prog2); err != nil {
		t.Fatalf("publish user-2 progress: %v", err)
	}
	if err := hub.UnregisterParticipant(ctx, matchID, "user-2"); err != nil {
		t.Fatalf("unregister user-2: %v", err)
	}

	// 4. Now both users are COMPLETED -> all Redis room keys must be purged
	progKey := "solo_match:" + matchID + ":progress"
	metaKey := "solo_match:" + matchID + ":exercise"
	if mr.Exists(partKey) {
		t.Errorf("expected partKey %s to be deleted after all participants finished", partKey)
	}
	if mr.Exists(progKey) {
		t.Errorf("expected progKey %s to be deleted after all participants finished", progKey)
	}
	if mr.Exists(metaKey) {
		t.Errorf("expected metaKey %s to be deleted after all participants finished", metaKey)
	}

	// 5. Test single-user abort/exit before match starts cleans up immediately
	singleMatchID := "match-single-exit"
	if err := hub.RegisterParticipant(ctx, singleMatchID, "user-solo", "squat_01"); err != nil {
		t.Fatalf("register user-solo: %v", err)
	}
	singlePartKey := "solo_match:" + singleMatchID + ":participants"
	if !mr.Exists(singlePartKey) {
		t.Fatalf("expected single participant key to exist")
	}
	if err := hub.UnregisterParticipant(ctx, singleMatchID, "user-solo"); err != nil {
		t.Fatalf("unregister user-solo: %v", err)
	}
	if mr.Exists(singlePartKey) {
		t.Errorf("expected single match room to be cleaned up when only user leaves")
	}
}
