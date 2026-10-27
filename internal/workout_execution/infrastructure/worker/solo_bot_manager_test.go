package worker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	domainEvent "github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
)

type mockBotPRRepo struct {
	records map[string]*aggregate.PersonalRecord
	err     error
}

var _ repository.PersonalRecordRepository = (*mockBotPRRepo)(nil)

func (m *mockBotPRRepo) Save(_ context.Context, _ *aggregate.PersonalRecord) error {
	return m.err
}

func (m *mockBotPRRepo) FindByUserIDAndExerciseID(_ context.Context, userID, exerciseID string) (*aggregate.PersonalRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := userID + ":" + exerciseID
	return m.records[key], nil
}

func (m *mockBotPRRepo) FindByUserIDAndExerciseIDForUpdate(_ context.Context, userID, exerciseID string) (*aggregate.PersonalRecord, error) {
	return m.FindByUserIDAndExerciseID(context.Background(), userID, exerciseID)
}

func (m *mockBotPRRepo) FindByUserIDAndExerciseIDs(_ context.Context, userID string, exerciseIDs []string) ([]*aggregate.PersonalRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	var res []*aggregate.PersonalRecord
	for _, id := range exerciseIDs {
		key := userID + ":" + id
		if r, ok := m.records[key]; ok {
			res = append(res, r)
		}
	}
	return res, nil
}

type mockBotLiveSyncHub struct {
	mu                     sync.Mutex
	registeredParticipants map[string]string
	publishedProgresses    []port.ParticipantProgress
	subCh                  chan port.ParticipantProgress
	readyOverride          bool
	regErr                 error
	pubErr                 error
}

var _ port.SoloMatchLiveSyncPort = (*mockBotLiveSyncHub)(nil)

func newMockBotLiveSyncHub() *mockBotLiveSyncHub {
	return &mockBotLiveSyncHub{
		registeredParticipants: make(map[string]string),
		subCh:                  make(chan port.ParticipantProgress, 10),
	}
}

func (m *mockBotLiveSyncHub) RegisterParticipant(_ context.Context, matchID, userID, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.regErr != nil {
		return m.regErr
	}
	m.registeredParticipants[userID] = matchID
	return nil
}

func (m *mockBotLiveSyncHub) UnregisterParticipant(_ context.Context, _, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.registeredParticipants, userID)
	return nil
}

func (m *mockBotLiveSyncHub) PublishMyProgress(_ context.Context, _ string, p port.ParticipantProgress) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pubErr != nil {
		return m.pubErr
	}
	m.publishedProgresses = append(m.publishedProgresses, p)
	return nil
}

func (m *mockBotLiveSyncHub) SubscribeMatchUpdates(_ context.Context, _, _ string) (<-chan port.ParticipantProgress, func(), error) {
	return m.subCh, func() {}, nil
}

func (m *mockBotLiveSyncHub) GetMyProgress(_ context.Context, _, _ string) (*port.ParticipantProgress, error) {
	return nil, nil
}

func (m *mockBotLiveSyncHub) GetOpponentProgress(_ context.Context, _, _ string) (*port.ParticipantProgress, error) {
	return nil, nil
}

func (m *mockBotLiveSyncHub) IsMatchReady(_ context.Context, _ string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readyOverride {
		return true, nil
	}
	return len(m.registeredParticipants) >= 2, nil
}

func (m *mockBotLiveSyncHub) DelRoom(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registeredParticipants = nil
	return nil
}

func (m *mockBotLiveSyncHub) ParticipantCount(_ context.Context, _ string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.registeredParticipants), nil
}

func (m *mockBotLiveSyncHub) Progresses() []port.ParticipantProgress {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]port.ParticipantProgress, len(m.publishedProgresses))
	copy(res, m.publishedProgresses)
	return res
}

func (m *mockBotLiveSyncHub) SetReady(ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readyOverride = ready
}

type mockBotOutboxWriter struct {
	mu     sync.Mutex
	events []interface{}
	err    error
}

var _ port.OutboxWriter = (*mockBotOutboxWriter)(nil)

func (m *mockBotOutboxWriter) WriteEvents(_ context.Context, _, _ string, events []interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, events...)
	return nil
}

func (m *mockBotOutboxWriter) Events() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]interface{}, len(m.events))
	copy(res, m.events)
	return res
}

func TestSoloBotManager(t *testing.T) {
	t.Run("StartBot ignores empty botUserID or matchID", func(t *testing.T) {
		bm := worker.NewSoloBotManager(nil, nil, nil)
		if err := bm.StartBot(context.Background(), "", "", "u1", "ex1", "REPS"); err != nil {
			t.Fatalf("expected nil error on empty ids, got %v", err)
		}
	})

	t.Run("StartBot with Reps challenge and opponent reaches target score", func(t *testing.T) {
		hub := newMockBotLiveSyncHub()
		hub.SetReady(true)

		pr := aggregate.NewPersonalRecord("pr1", "u1", "ex-squat", 0, 3, true, time.Now().UTC())
		prRepo := &mockBotPRRepo{
			records: map[string]*aggregate.PersonalRecord{
				"u1:ex-squat": pr,
			},
		}
		outbox := &mockBotOutboxWriter{}

		bm := worker.NewSoloBotManager(prRepo, hub, outbox)
		bm.SetRunnerDelaysForTesting(10*time.Millisecond, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := bm.StartBot(ctx, "match-123", "bot-001", "u1", "ex-squat", "REPS")
		if err != nil {
			t.Fatalf("unexpected error starting bot: %v", err)
		}

		// Chờ bot đếm xong 3 reps
		var finished bool
		for i := 0; i < 50; i++ {
			time.Sleep(20 * time.Millisecond)
			evts := outbox.Events()
			if len(evts) > 0 {
				finished = true
				comp, ok := evts[0].(*domainEvent.SoloTrackingCompleted)
				if !ok {
					t.Fatalf("expected SoloTrackingCompleted event, got %T", evts[0])
				}
				if comp.ConfirmedValue != 3 {
					t.Errorf("expected score 3 from PR, got %d", comp.ConfirmedValue)
				}
				if comp.UserID != "bot-001" {
					t.Errorf("expected bot-001, got %s", comp.UserID)
				}
				break
			}
		}
		if !finished {
			t.Fatal("timed out waiting for bot runner to complete reps challenge")
		}
	})

	t.Run("StartBot with Duration challenge completes target seconds", func(t *testing.T) {
		hub := newMockBotLiveSyncHub()
		hub.SetReady(true)

		pr := aggregate.NewPersonalRecord("pr2", "u1", "ex-plank", 0, 2, true, time.Now().UTC())
		prRepo := &mockBotPRRepo{
			records: map[string]*aggregate.PersonalRecord{
				"u1:ex-plank": pr,
			},
		}
		outbox := &mockBotOutboxWriter{}

		bm := worker.NewSoloBotManager(prRepo, hub, outbox)
		bm.SetRunnerDelaysForTesting(5*time.Millisecond, 5*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := bm.StartBot(ctx, "match-dur", "bot-002", "u1", "ex-plank", "DURATION")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var finished bool
		for i := 0; i < 50; i++ {
			time.Sleep(20 * time.Millisecond)
			evts := outbox.Events()
			if len(evts) > 0 {
				finished = true
				comp, ok := evts[0].(*domainEvent.SoloTrackingCompleted)
				if !ok {
					t.Fatalf("expected *domainEvent.SoloTrackingCompleted, got %T", evts[0])
				}
				if comp.ChallengeType != vo.ChallengeTypeDuration {
					t.Errorf("expected duration challenge type, got %v", comp.ChallengeType)
				}
				break
			}
		}
		if !finished {
			t.Fatal("timed out waiting for bot to complete duration challenge")
		}
	})

	t.Run("CancelBot stops specific bot runner cleanly", func(t *testing.T) {
		hub := newMockBotLiveSyncHub()
		hub.SetReady(false) // Giữ ở trạng thái chờ

		outbox := &mockBotOutboxWriter{}
		bm := worker.NewSoloBotManager(nil, hub, outbox)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_ = bm.StartBot(ctx, "match-cancel", "bot-003", "u1", "ex-pushup", "REPS")
		time.Sleep(20 * time.Millisecond)

		bm.CancelBot("match-cancel")

		// Verify bot unregisters and does not emit completed
		time.Sleep(50 * time.Millisecond)
		if len(outbox.Events()) > 0 {
			t.Fatal("expected no completed events for cancelled bot")
		}
	})

	t.Run("StopAll cancels all active bots cleanly and waits", func(t *testing.T) {
		hub := newMockBotLiveSyncHub()
		hub.SetReady(false)

		outbox := &mockBotOutboxWriter{}
		bm := worker.NewSoloBotManager(nil, hub, outbox)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_ = bm.StartBot(ctx, "match-all-1", "bot-a", "u1", "ex-1", "REPS")
		_ = bm.StartBot(ctx, "match-all-2", "bot-b", "u2", "ex-2", "REPS")

		time.Sleep(20 * time.Millisecond)

		done := make(chan struct{})
		go func() {
			bm.StopAll()
			close(done)
		}()

		select {
		case <-done:
			// OK
		case <-time.After(2 * time.Second):
			t.Fatal("StopAll timed out")
		}
	})

	t.Run("Opponent abort completes bot early", func(t *testing.T) {
		hub := newMockBotLiveSyncHub()
		hub.SetReady(true)

		outbox := &mockBotOutboxWriter{}
		bm := worker.NewSoloBotManager(nil, hub, outbox)
		bm.SetRunnerDelaysForTesting(100*time.Millisecond, 100*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_ = bm.StartBot(ctx, "match-abort", "bot-004", "u1", "ex-squat", "REPS")
		time.Sleep(20 * time.Millisecond)

		// Opponent aborts
		hub.subCh <- port.ParticipantProgress{
			UserID: "u1",
			Status: "ABORTED",
		}

		var finished bool
		for i := 0; i < 30; i++ {
			time.Sleep(20 * time.Millisecond)
			if len(outbox.Events()) > 0 {
				finished = true
				break
			}
		}
		if !finished {
			t.Fatal("expected bot to complete early on opponent abort")
		}
	})

	t.Run("Opponent disconnect does not stop bot early during grace period", func(t *testing.T) {
		hub := newMockBotLiveSyncHub()
		hub.SetReady(true)

		outbox := &mockBotOutboxWriter{}
		bm := worker.NewSoloBotManager(nil, hub, outbox)
		bm.SetRunnerDelaysForTesting(100*time.Millisecond, 100*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_ = bm.StartBot(ctx, "match-disc", "bot-005", "u1", "ex-squat", "REPS")
		time.Sleep(20 * time.Millisecond)

		// Opponent disconnects (grace period starts)
		hub.subCh <- port.ParticipantProgress{
			UserID: "u1",
			Status: "DISCONNECTED",
		}

		// Wait briefly: bot should NOT have completed early
		time.Sleep(50 * time.Millisecond)
		if len(outbox.Events()) > 0 {
			t.Fatal("expected bot NOT to complete early on opponent temporary disconnect")
		}

		// Opponent reconnects
		hub.subCh <- port.ParticipantProgress{
			UserID: "u1",
			Status: "ACTIVE",
		}

		time.Sleep(20 * time.Millisecond)
		if len(outbox.Events()) > 0 {
			t.Fatal("expected bot still exercising after opponent reconnected")
		}

		// Now opponent actually aborts (e.g. grace period expired)
		hub.subCh <- port.ParticipantProgress{
			UserID: "u1",
			Status: "ABORTED",
		}

		var finished bool
		for i := 0; i < 30; i++ {
			time.Sleep(20 * time.Millisecond)
			if len(outbox.Events()) > 0 {
				finished = true
				break
			}
		}
		if !finished {
			t.Fatal("expected bot to complete early on opponent abort")
		}
	})
}
