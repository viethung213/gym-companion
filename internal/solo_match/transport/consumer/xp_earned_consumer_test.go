package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/consumer"
)

func TestXpEarnedConsumer(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when payload is invalid json", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		c := consumer.NewXpEarnedConsumer(handler, nil)

		if err := c.HandleMessage(ctx, []byte(`not-json`)); err == nil {
			t.Fatalf("expected error for invalid json, got nil")
		}
	})

	t.Run("ignores events that are not XpEarned", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		c := consumer.NewXpEarnedConsumer(handler, nil)

		payload := []byte(`{"id":"evt_1","type":"contracts.supporting.gamification.v1.userLeveledUp","data":{}}`)
		if err := c.HandleMessage(ctx, payload); err != nil {
			t.Fatalf("expected nil for ignored event, got %v", err)
		}
		if len(repo.snapshots) != 0 {
			t.Fatalf("expected no snapshots updated")
		}
	})

	t.Run("skips already processed event (idempotency)", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: map[string]bool{"evt_xp_done": true},
			saved:     make(map[string]string),
		}
		c := consumer.NewXpEarnedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_xp_done",
			"type": "contracts.supporting.gamification.v1.xpEarned",
			"data": {"userId":"u_1","newXp":"1250"}
		}`)
		if err := c.HandleMessage(ctx, payload); err != nil {
			t.Fatalf("expected nil for idempotent event, got %v", err)
		}
		if len(repo.snapshots) != 0 {
			t.Fatalf("expected handler not to run for idempotent event")
		}
	})

	t.Run("processes protojson XpEarned event with string int64 newXp", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewXpEarnedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_xp_1",
			"type": "contracts.supporting.gamification.v1.xpEarned",
			"data": {
				"userId": "u_xp_1",
				"deltaXp": 50,
				"newXp": "1250",
				"newLevel": 4,
				"source": "WORKOUT_COMPLETED",
				"earnedAt": "2026-11-14T10:00:00Z"
			}
		}`)
		if err := c.HandleMessage(ctx, payload); err != nil {
			t.Fatalf("HandleMessage failed: %v", err)
		}

		snap := repo.snapshots["u_xp_1"]
		if snap == nil || snap.XP() != 1250 {
			t.Fatalf("expected xp 1250, got %+v", snap)
		}
		if logRepo.saved["evt_xp_1"] != "PROCESSED" {
			t.Errorf("expected outbox log PROCESSED, got %s", logRepo.saved["evt_xp_1"])
		}
	})

	t.Run("logs FAILED when handler returns error", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{
			snapshots: make(map[string]*entity.UserSnapshot),
			err:       errors.New("db failure"),
		}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewXpEarnedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_xp_err",
			"type": "contracts.supporting.gamification.v1.xpEarned",
			"data": {"userId":"u_err","newXp":"500"}
		}`)
		if err := c.HandleMessage(ctx, payload); err == nil {
			t.Fatalf("expected error, got nil")
		}
		if logRepo.saved["evt_xp_err"] != "FAILED" {
			t.Errorf("expected FAILED log, got %s", logRepo.saved["evt_xp_err"])
		}
	})
}
