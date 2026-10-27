package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/consumer"
)

func TestWorkoutAbortedConsumer(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when payload is invalid json", func(t *testing.T) {
		repo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantAbortHandler(repo, &mockEventPub{}, &mockTxManager{}, nil)
		c := consumer.NewWorkoutAbortedConsumer(handler, nil)

		err := c.HandleMessage(ctx, []byte(`invalid json`))
		if err == nil {
			t.Errorf("expected error for invalid json, got nil")
		}
	})

	t.Run("ignores events that are not workout aborted or solo tracking aborted", func(t *testing.T) {
		repo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantAbortHandler(repo, &mockEventPub{}, &mockTxManager{}, nil)
		c := consumer.NewWorkoutAbortedConsumer(handler, nil)

		payload := []byte(`{
			"id": "evt_1",
			"type": "contracts.core.workout_execution.v1.workoutSessionStarted",
			"data": {}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil for ignored event type, got: %v", err)
		}
	})

	t.Run("skips already processed event (idempotency)", func(t *testing.T) {
		repo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantAbortHandler(repo, &mockEventPub{}, &mockTxManager{}, nil)
		logRepo := &mockOutboxLogRepo{
			processed: map[string]bool{"evt_already_done": true},
			saved:     make(map[string]string),
		}
		c := consumer.NewWorkoutAbortedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_already_done",
			"type": "contracts.core.workout_execution.v1.soloTrackingAborted",
			"data": {
				"matchId": "m_1",
				"userId": "u_1",
				"reason": "USER_FORFEITED"
			}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil for already processed event: %v", err)
		}
		if len(repo.matches) != 0 {
			t.Errorf("expected no match update for idempotent event")
		}
	})

	t.Run("successfully processes SoloTrackingAborted with camelCase", func(t *testing.T) {
		match := makeTestMatch("m_200", "u_alice", "u_bob")
		repo := &mockMatchRepo{matches: map[string]*aggregate.SoloMatch{"m_200": match}}
		handler := command.NewRecordParticipantAbortHandler(repo, &mockEventPub{}, &mockTxManager{}, nil)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewWorkoutAbortedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_abort_1",
			"type": "contracts.core.workout_execution.v1.soloTrackingAborted",
			"data": {
				"matchId": "m_200",
				"userId": "u_alice",
				"reason": "USER_FORFEITED"
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if logRepo.saved["evt_abort_1"] != "PROCESSED" {
			t.Errorf("expected outbox log PROCESSED, got %s", logRepo.saved["evt_abort_1"])
		}

		if match.Status() != valueobject.MatchStatusForfeited {
			t.Errorf("expected match forfeited after forfeit, got %v", match.Status())
		}
		if match.WinnerID() == nil || *match.WinnerID() != "u_bob" {
			t.Errorf("expected winner u_bob, got %v", match.WinnerID())
		}
	})

	t.Run("successfully processes SoloTrackingAborted with EventName type", func(t *testing.T) {
		match := makeTestMatch("m_201", "u_alice", "u_bob")
		repo := &mockMatchRepo{matches: map[string]*aggregate.SoloMatch{"m_201": match}}
		handler := command.NewRecordParticipantAbortHandler(repo, &mockEventPub{}, &mockTxManager{}, nil)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewWorkoutAbortedConsumer(handler, logRepo)

		payload := []byte(`{
			"specversion": "1.0",
			"id": "evt_abort_2",
			"source": "services/workout-execution-service",
			"type": "contracts.core.workout_execution.v1.event.SoloTrackingAborted",
			"time": "2026-10-25T10:00:00Z",
			"datacontenttype": "application/json",
			"data": {
				"matchId": "m_201",
				"userId": "u_bob",
				"reason": "USER_DISCONNECTED"
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if logRepo.saved["evt_abort_2"] != "PROCESSED" {
			t.Errorf("expected outbox log PROCESSED, got %s", logRepo.saved["evt_abort_2"])
		}

		if match.Status() != valueobject.MatchStatusForfeited {
			t.Errorf("expected match forfeited after forfeit, got %v", match.Status())
		}
		if match.WinnerID() == nil || *match.WinnerID() != "u_alice" {
			t.Errorf("expected winner u_alice, got %v", match.WinnerID())
		}
	})

	t.Run("ignores legacy WorkoutSessionAborted", func(t *testing.T) {
		repo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantAbortHandler(repo, &mockEventPub{}, &mockTxManager{}, nil)
		c := consumer.NewWorkoutAbortedConsumer(handler, nil)

		payload := []byte(`{
			"id": "evt_legacy_abort_1",
			"type": "contracts.core.workout_execution.v1.workoutSessionAborted",
			"data": {
				"matchId": "m_202",
				"userId": "u_bob",
				"reason": "workout_session_aborted"
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if len(repo.matches) != 0 {
			t.Errorf("expected legacy event to be ignored")
		}
	})

	t.Run("skips when matchId is missing", func(t *testing.T) {
		repo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantAbortHandler(repo, &mockEventPub{}, &mockTxManager{}, nil)
		c := consumer.NewWorkoutAbortedConsumer(handler, nil)

		payload := []byte(`{
			"id": "evt_no_match",
			"type": "contracts.core.workout_execution.v1.soloTrackingAborted",
			"data": {
				"userId": "u_1",
				"reason": "USER_FORFEITED"
			}
		}`)
		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil for session without matchId, got: %v", err)
		}
	})

	t.Run("records FAILED in outbox log when handler returns error", func(t *testing.T) {
		repo := &mockMatchRepo{
			matches: make(map[string]*aggregate.SoloMatch),
			err:     errors.New("db error"),
		}
		handler := command.NewRecordParticipantAbortHandler(repo, &mockEventPub{}, &mockTxManager{}, nil)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewWorkoutAbortedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_fail_abort_1",
			"type": "contracts.core.workout_execution.v1.soloTrackingAborted",
			"data": {
				"matchId": "m_not_found",
				"userId": "u_alice",
				"reason": "USER_FORFEITED"
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err == nil {
			t.Fatalf("expected handler error, got nil")
		}

		if logRepo.saved["evt_fail_abort_1"] != "FAILED" {
			t.Errorf("expected outbox log FAILED, got %s", logRepo.saved["evt_fail_abort_1"])
		}
	})
}
