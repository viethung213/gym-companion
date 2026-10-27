package consumer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/consumer"
)

type mockMatchRepo struct {
	matches map[string]*aggregate.SoloMatch
	err     error
}

func (m *mockMatchRepo) Save(ctx context.Context, match *aggregate.SoloMatch) error {
	if m.err != nil {
		return m.err
	}
	m.matches[match.ID()] = match
	return nil
}

func (m *mockMatchRepo) FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.matches[id], nil
}

func (m *mockMatchRepo) FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *mockMatchRepo) FindUnreadyExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *mockMatchRepo) FindExpiredInProgressMatches(ctx context.Context, startedBefore time.Time, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *mockMatchRepo) ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

type mockEventPub struct {
	events []any
}

func (m *mockEventPub) PublishEvents(ctx context.Context, events []any) error {
	m.events = append(m.events, events...)
	return nil
}

type mockTxManager struct{}

func (t *mockTxManager) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

func makeTestMatch(matchID, user1, user2 string) *aggregate.SoloMatch {
	now := time.Now().UTC()
	p1, _ := entity.NewMatchParticipant("p_1", matchID, user1)
	p2, _ := entity.NewMatchParticipant("p_2", matchID, user2)
	match, _ := aggregate.NewSoloMatch(
		matchID,
		"ex_squat",
		"Squat",
		valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	_ = match.ActivateRoom(now)
	return match
}

func TestWorkoutCompletedConsumer(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when payload is invalid json", func(t *testing.T) {
		repo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantResultHandler(repo, &mockEventPub{}, &mockTxManager{}, nil, nil)
		c := consumer.NewWorkoutCompletedConsumer(handler, nil)

		err := c.HandleMessage(ctx, []byte(`invalid json`))
		if err == nil {
			t.Errorf("expected error for invalid json, got nil")
		}
	})

	t.Run("ignores events that are not workout completed or solo tracking completed", func(t *testing.T) {
		repo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantResultHandler(repo, &mockEventPub{}, &mockTxManager{}, nil, nil)
		c := consumer.NewWorkoutCompletedConsumer(handler, nil)

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
		handler := command.NewRecordParticipantResultHandler(repo, &mockEventPub{}, &mockTxManager{}, nil, nil)
		logRepo := &mockOutboxLogRepo{
			processed: map[string]bool{"evt_already_done": true},
			saved:     make(map[string]string),
		}
		c := consumer.NewWorkoutCompletedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_already_done",
			"type": "contracts.core.workout_execution.v1.soloTrackingCompleted",
			"data": {
				"matchId": "m_1",
				"userId": "u_1",
				"confirmedValue": 30
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

	t.Run("successfully processes SoloTrackingCompleted with camelCase", func(t *testing.T) {
		match := makeTestMatch("m_100", "u_alice", "u_bob")
		repo := &mockMatchRepo{matches: map[string]*aggregate.SoloMatch{"m_100": match}}
		handler := command.NewRecordParticipantResultHandler(repo, &mockEventPub{}, &mockTxManager{}, nil, nil)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewWorkoutCompletedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_solo_1",
			"type": "contracts.core.workout_execution.v1.soloTrackingCompleted",
			"data": {
				"matchId": "m_100",
				"userId": "u_alice",
				"confirmedValue": 42,
				"completedAt": "2026-10-25T10:00:00Z"
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if logRepo.saved["evt_solo_1"] != "PROCESSED" {
			t.Errorf("expected outbox log PROCESSED, got %s", logRepo.saved["evt_solo_1"])
		}

		p1 := match.Participants()[0]
		if p1.Score().Value() != 42 {
			t.Errorf("expected participant score 42, got %v", p1.Score().Value())
		}
	})

	t.Run("successfully processes SoloTrackingCompleted with EventName type", func(t *testing.T) {
		match := makeTestMatch("m_101", "u_alice", "u_bob")
		repo := &mockMatchRepo{matches: map[string]*aggregate.SoloMatch{"m_101": match}}
		handler := command.NewRecordParticipantResultHandler(repo, &mockEventPub{}, &mockTxManager{}, nil, nil)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewWorkoutCompletedConsumer(handler, logRepo)

		payload := []byte(`{
			"specversion": "1.0",
			"id": "evt_solo_2",
			"source": "services/workout-execution-service",
			"type": "contracts.core.workout_execution.v1.event.SoloTrackingCompleted",
			"time": "2026-10-25T10:00:00Z",
			"datacontenttype": "application/json",
			"data": {
				"matchId": "m_101",
				"userId": "u_bob",
				"confirmedValue": 35,
				"completedAt": "2026-10-25T10:00:00Z"
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		if logRepo.saved["evt_solo_2"] != "PROCESSED" {
			t.Errorf("expected outbox log PROCESSED, got %s", logRepo.saved["evt_solo_2"])
		}

		p2 := match.Participants()[1]
		if p2.Score().Value() != 35 {
			t.Errorf("expected participant score 35, got %v", p2.Score().Value())
		}
	})

	t.Run("ignores legacy WorkoutSessionCompleted", func(t *testing.T) {
		repo := &mockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
		handler := command.NewRecordParticipantResultHandler(repo, &mockEventPub{}, &mockTxManager{}, nil, nil)
		c := consumer.NewWorkoutCompletedConsumer(handler, nil)

		payload := []byte(`{
			"id": "evt_legacy_1",
			"type": "contracts.core.workout_execution.v1.workoutSessionCompleted",
			"data": {
				"matchId": "m_102",
				"userId": "u_alice",
				"scoreValue": 25
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
		handler := command.NewRecordParticipantResultHandler(repo, &mockEventPub{}, &mockTxManager{}, nil, nil)
		c := consumer.NewWorkoutCompletedConsumer(handler, nil)

		payload := []byte(`{
			"id": "evt_no_match",
			"type": "contracts.core.workout_execution.v1.soloTrackingCompleted",
			"data": {
				"userId": "u_1",
				"confirmedValue": 30
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
		handler := command.NewRecordParticipantResultHandler(repo, &mockEventPub{}, &mockTxManager{}, nil, nil)
		logRepo := &mockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
		}
		c := consumer.NewWorkoutCompletedConsumer(handler, logRepo)

		payload := []byte(`{
			"id": "evt_fail_1",
			"type": "contracts.core.workout_execution.v1.soloTrackingCompleted",
			"data": {
				"matchId": "m_not_found",
				"userId": "u_alice",
				"confirmedValue": 20
			}
		}`)

		err := c.HandleMessage(ctx, payload)
		if err == nil {
			t.Fatalf("expected handler error, got nil")
		}

		if logRepo.saved["evt_fail_1"] != "FAILED" {
			t.Errorf("expected outbox log FAILED, got %s", logRepo.saved["evt_fail_1"])
		}
	})
}
