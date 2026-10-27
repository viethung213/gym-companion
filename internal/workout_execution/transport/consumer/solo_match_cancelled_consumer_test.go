package consumer_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/transport/consumer"
)

type mockBotCanceler struct {
	cancelledMatchID string
}

func (m *mockBotCanceler) CancelBot(matchID string) {
	m.cancelledMatchID = matchID
}

type mockCancelledLiveSync struct {
	mu                  sync.Mutex
	publishedProgresses []port.ParticipantProgress
	deletedRooms        []string
}

var _ port.SoloMatchLiveSyncPort = (*mockCancelledLiveSync)(nil)

func (m *mockCancelledLiveSync) RegisterParticipant(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockCancelledLiveSync) UnregisterParticipant(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockCancelledLiveSync) PublishMyProgress(_ context.Context, _ string, p port.ParticipantProgress) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishedProgresses = append(m.publishedProgresses, p)
	return nil
}

func (m *mockCancelledLiveSync) SubscribeMatchUpdates(_ context.Context, _, _ string) (<-chan port.ParticipantProgress, func(), error) {
	return nil, func() {}, nil
}

func (m *mockCancelledLiveSync) GetMyProgress(_ context.Context, _, _ string) (*port.ParticipantProgress, error) {
	return nil, nil
}

func (m *mockCancelledLiveSync) GetOpponentProgress(_ context.Context, _, _ string) (*port.ParticipantProgress, error) {
	return nil, nil
}

func (m *mockCancelledLiveSync) IsMatchReady(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockCancelledLiveSync) DelRoom(_ context.Context, matchID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedRooms = append(m.deletedRooms, matchID)
	return nil
}

func (m *mockCancelledLiveSync) ParticipantCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func TestSoloMatchCancelledConsumer_Success(t *testing.T) {
	ctx := context.Background()
	liveSync := &mockCancelledLiveSync{}
	botCanceler := &mockBotCanceler{}

	c := consumer.NewSoloMatchCancelledConsumer(liveSync, botCanceler, nil)

	payload := map[string]interface{}{
		"specversion": "1.0",
		"id":          "evt_cancel_123",
		"source":      "/solo_match",
		"type":        "contracts.core.solo_match.v1.soloMatchCancelled",
		"data": map[string]interface{}{
			"matchId": "match_cancel_1",
			"reason":  "NO_SHOW_TIMEOUT",
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	if err := c.HandleMessage(ctx, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if botCanceler.cancelledMatchID != "match_cancel_1" {
		t.Errorf("expected bot cancel for match_cancel_1, got %q", botCanceler.cancelledMatchID)
	}

	if len(liveSync.deletedRooms) != 1 || liveSync.deletedRooms[0] != "match_cancel_1" {
		t.Errorf("expected deleted room match_cancel_1, got %+v", liveSync.deletedRooms)
	}
}

func TestSoloMatchCancelledConsumer_IgnoredUnrelatedEvent(t *testing.T) {
	ctx := context.Background()
	liveSync := &mockCancelledLiveSync{}
	botCanceler := &mockBotCanceler{}

	c := consumer.NewSoloMatchCancelledConsumer(liveSync, botCanceler, nil)

	payload := map[string]interface{}{
		"type": "contracts.core.user.v1.userRegistered",
	}
	raw, _ := json.Marshal(payload)

	if err := c.HandleMessage(ctx, raw); err != nil {
		t.Fatalf("unexpected error for unrelated event: %v", err)
	}
	if botCanceler.cancelledMatchID != "" {
		t.Errorf("expected no cancellation, got %q", botCanceler.cancelledMatchID)
	}
}
