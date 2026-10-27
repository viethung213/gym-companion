package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/consumer"
)

func TestUserRegisteredConsumer(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when payload is invalid json", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		c := consumer.NewUserRegisteredConsumer(handler, nil)

		err := c.HandleMessage(ctx, []byte(`invalid json`))
		if err == nil {
			t.Errorf("expected error for invalid json, got nil")
		}
	})

	t.Run("ignores events that are not UserRegistered", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		c := consumer.NewUserRegisteredConsumer(handler, nil)

		payload := []byte(`{
			"id": "evt_1",
			"type": "contracts.core.workout.v1.event.WorkoutCompleted",
			"data": {}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil for ignored event type, got: %v", err)
		}
		if len(repo.snapshots) != 0 {
			t.Errorf("expected no snapshots to be updated")
		}
	})

	t.Run("skips already processed event (idempotency)", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: map[string]bool{"evt_already_done": true},
			saved:     make(map[string]string),
		}
		c := consumer.NewUserRegisteredConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_already_done",
			"type": "contracts.supporting.auth.v1.event.UserRegistered",
			"data": {
				"userId": "u_done",
				"fullName": "Alice"
			}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil for already processed event: %v", err)
		}
		if len(repo.snapshots) != 0 {
			t.Errorf("expected no snapshots to be updated")
		}
	})

	t.Run("skips when userId is empty", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		c := consumer.NewUserRegisteredConsumer(handler, nil)

		payload := []byte(`{
			"id": "evt_no_user",
			"type": "contracts.supporting.auth.v1.event.UserRegistered",
			"data": {
				"fullName": "Alice"
			}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil error when userId empty, got: %v", err)
		}
	})

	t.Run("successfully processes UserRegistered event and logs as PROCESSED", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{snapshots: make(map[string]*entity.UserSnapshot)}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewUserRegisteredConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_reg_1",
			"type": "contracts.supporting.auth.v1.event.UserRegistered",
			"data": {
				"userId": "u_new",
				"fullName": "New User",
				"avatarUrl": "https://avatar.png/new",
				"registeredAt": "2026-10-25T08:00:00Z"
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if logRepo.saved["evt_reg_1"] != "PROCESSED" {
			t.Errorf("expected status PROCESSED, got: %s", logRepo.saved["evt_reg_1"])
		}

		snap, ok := repo.snapshots["u_new"]
		if !ok || snap == nil {
			t.Fatalf("expected snapshot to be saved")
		}
		if snap.DisplayName() != "New User" {
			t.Errorf("expected displayName 'New User', got: %s", snap.DisplayName())
		}
	})

	t.Run("logs as FAILED and returns error when handler fails", func(t *testing.T) {
		repo := &mockUserSnapshotRepo{
			snapshots: make(map[string]*entity.UserSnapshot),
			err:       errors.New("db error"),
		}
		handler := command.NewSyncUserSnapshotHandler(repo)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewUserRegisteredConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_fail_reg",
			"type": "contracts.supporting.auth.v1.event.UserRegistered",
			"data": {
				"userId": "u_err",
				"fullName": "Error User"
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err == nil {
			t.Fatalf("expected error from handler, got nil")
		}

		if logRepo.saved["evt_fail_reg"] != "FAILED" {
			t.Errorf("expected status FAILED, got: %s", logRepo.saved["evt_fail_reg"])
		}
	})
}
