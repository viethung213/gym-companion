package consumer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/transport/consumer"
)

type startBotCall struct {
	MatchID       string
	BotUserID     string
	UserPlayerID  string
	ExerciseID    string
	ChallengeType string
}

type mockConsumerBotManager struct {
	mu           sync.Mutex
	startedCalls []startBotCall
	err          error
}

var _ port.SoloBotManager = (*mockConsumerBotManager)(nil)

func (m *mockConsumerBotManager) StartBot(_ context.Context, matchID, botUserID, userPlayerID, exerciseID, challengeType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startedCalls = append(m.startedCalls, startBotCall{
		MatchID:       matchID,
		BotUserID:     botUserID,
		UserPlayerID:  userPlayerID,
		ExerciseID:    exerciseID,
		ChallengeType: challengeType,
	})
	return m.err
}

func (m *mockConsumerBotManager) CancelBot(_ string) {}

func (m *mockConsumerBotManager) StopAll() {}

func (m *mockConsumerBotManager) SetRunnerDelaysForTesting(_, _ time.Duration) {}

func (m *mockConsumerBotManager) Calls() []startBotCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]startBotCall, len(m.startedCalls))
	copy(res, m.startedCalls)
	return res
}

type mockConsumerOutboxLogRepo struct {
	mu        sync.Mutex
	processed map[string]bool
	savedLogs []*port.OutboxLogRecord
}

var _ port.OutboxLogRepository = (*mockConsumerOutboxLogRepo)(nil)

func (m *mockConsumerOutboxLogRepo) IsProcessed(_ context.Context, eventID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.processed[eventID], nil
}

func (m *mockConsumerOutboxLogRepo) SaveLog(_ context.Context, record *port.OutboxLogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savedLogs = append(m.savedLogs, record)
	return nil
}

func (m *mockConsumerOutboxLogRepo) Logs() []*port.OutboxLogRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]*port.OutboxLogRecord, len(m.savedLogs))
	copy(res, m.savedLogs)
	return res
}

func TestSoloMatchStartedConsumer(t *testing.T) {
	t.Run("ignore non matching event type", func(t *testing.T) {
		c := consumer.NewSoloMatchStartedConsumer(nil, nil)
		err := c.HandleMessage(context.Background(), []byte(`{"type":"contracts.core.user.v1.userRegistered"}`))
		if err != nil {
			t.Fatalf("expected nil error on ignored event type, got %v", err)
		}
	})

	t.Run("skip already processed event in outbox log", func(t *testing.T) {
		outboxLog := &mockConsumerOutboxLogRepo{
			processed: map[string]bool{"evt_already_done": true},
		}
		bm := &mockConsumerBotManager{}
		c := consumer.NewSoloMatchStartedConsumer(bm, outboxLog)
		payload := []byte(`{"id":"evt_already_done","type":"contracts.core.solo_match.v1.soloMatchStarted","data":{"matchId":"m1"}}`)
		if err := c.HandleMessage(context.Background(), payload); err != nil {
			t.Fatalf("expected nil error when skipping, got %v", err)
		}
		if len(bm.Calls()) != 0 {
			t.Fatalf("expected 0 bot manager calls on skipped event")
		}
	})

	t.Run("ignore PvP match without bot", func(t *testing.T) {
		outboxLog := &mockConsumerOutboxLogRepo{
			processed: make(map[string]bool),
		}
		bm := &mockConsumerBotManager{}
		c := consumer.NewSoloMatchStartedConsumer(bm, outboxLog)
		payload := []byte(`{
			"id": "evt_pvp_1",
			"type": "contracts.core.solo_match.v1.soloMatchStarted",
			"data": {
				"matchId": "m_pvp",
				"exerciseId": "ex_pushup",
				"player1Id": "u1",
				"player2Id": "u2",
				"botUserId": ""
			}
		}`)
		if err := c.HandleMessage(context.Background(), payload); err != nil {
			t.Fatalf("expected nil error for pvp match, got %v", err)
		}
		if len(bm.Calls()) != 0 {
			t.Fatalf("expected 0 bot manager calls for pvp match")
		}
		logs := outboxLog.Logs()
		if len(logs) != 1 || logs[0].Status != "PROCESSED" {
			t.Fatalf("expected 1 PROCESSED log record for pvp match, got %+v", logs)
		}
	})

	t.Run("delegates bot start to SoloBotManager when botUserId is present", func(t *testing.T) {
		outboxLog := &mockConsumerOutboxLogRepo{
			processed: make(map[string]bool),
		}
		bm := &mockConsumerBotManager{}
		c := consumer.NewSoloMatchStartedConsumer(bm, outboxLog)

		payload := []byte(`{
			"id": "evt_bot_match_101",
			"type": "contracts.core.solo_match.v1.soloMatchStarted",
			"data": {
				"matchId": "match_stream_101",
				"exerciseId": "ex_pullup",
				"exerciseName": "Pull-up",
				"challengeType": "REPS",
				"player1Id": "user_athlete",
				"player2Id": "bot_pro",
				"botUserId": "bot_pro"
			}
		}`)

		if err := c.HandleMessage(context.Background(), payload); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		calls := bm.Calls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call to botManager.StartBot, got %d", len(calls))
		}
		call := calls[0]
		if call.MatchID != "match_stream_101" || call.BotUserID != "bot_pro" || call.UserPlayerID != "user_athlete" || call.ExerciseID != "ex_pullup" || call.ChallengeType != "REPS" {
			t.Fatalf("unexpected call parameters: %+v", call)
		}

		logs := outboxLog.Logs()
		if len(logs) != 1 || logs[0].Status != "PROCESSED" {
			t.Fatalf("expected 1 PROCESSED log record, got %+v", logs)
		}
	})

	t.Run("resolves userPlayerID when bot is player1", func(t *testing.T) {
		outboxLog := &mockConsumerOutboxLogRepo{
			processed: make(map[string]bool),
		}
		bm := &mockConsumerBotManager{}
		c := consumer.NewSoloMatchStartedConsumer(bm, outboxLog)

		payload := []byte(`{
			"id": "evt_bot_p1",
			"type": "contracts.core.solo_match.v1.soloMatchStarted",
			"data": {
				"matchId": "match_p1",
				"exerciseId": "ex_squat",
				"challengeType": "DURATION",
				"player1Id": "bot_pro",
				"player2Id": "user_athlete",
				"botUserId": "bot_pro"
			}
		}`)

		if err := c.HandleMessage(context.Background(), payload); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		calls := bm.Calls()
		if len(calls) != 1 {
			t.Fatalf("expected 1 call to botManager, got %d", len(calls))
		}
		if calls[0].UserPlayerID != "user_athlete" {
			t.Fatalf("expected userPlayerID user_athlete, got %s", calls[0].UserPlayerID)
		}
	})

	t.Run("records FAILED outbox log when botManager returns error", func(t *testing.T) {
		outboxLog := &mockConsumerOutboxLogRepo{
			processed: make(map[string]bool),
		}
		bm := &mockConsumerBotManager{
			err: errors.New("redis down"),
		}
		c := consumer.NewSoloMatchStartedConsumer(bm, outboxLog)

		payload := []byte(`{
			"id": "evt_bot_err",
			"type": "contracts.core.solo_match.v1.soloMatchStarted",
			"data": {
				"matchId": "match_err",
				"exerciseId": "ex_pushup",
				"player1Id": "u1",
				"player2Id": "bot_1",
				"botUserId": "bot_1"
			}
		}`)

		err := c.HandleMessage(context.Background(), payload)
		if err == nil {
			t.Fatalf("expected error from botManager, got nil")
		}

		logs := outboxLog.Logs()
		if len(logs) != 1 || logs[0].Status != "FAILED" || logs[0].ErrorMessage != "redis down" {
			t.Fatalf("expected 1 FAILED log with error message, got %+v", logs)
		}
	})

	t.Run("returns error on invalid json", func(t *testing.T) {
		c := consumer.NewSoloMatchStartedConsumer(nil, nil)
		err := c.HandleMessage(context.Background(), []byte(`invalid json`))
		if err == nil {
			t.Fatalf("expected unmarshal error on malformed json")
		}
	})
}
