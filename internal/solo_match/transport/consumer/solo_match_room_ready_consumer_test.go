package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/consumer"
)

type roomReadyMockMatchRepo struct {
	matches map[string]*aggregate.SoloMatch
	err     error
}

func (m *roomReadyMockMatchRepo) Save(_ context.Context, match *aggregate.SoloMatch) error {
	if m.err != nil {
		return m.err
	}
	m.matches[match.ID()] = match
	return nil
}

func (m *roomReadyMockMatchRepo) FindByID(_ context.Context, id string) (*aggregate.SoloMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.matches[id], nil
}

func (m *roomReadyMockMatchRepo) FindActiveByUserID(_ context.Context, _ string) (*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *roomReadyMockMatchRepo) FindUnreadyExpiredMatches(_ context.Context, _ time.Time, _ int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *roomReadyMockMatchRepo) FindExpiredInProgressMatches(_ context.Context, _ time.Time, _ int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func (m *roomReadyMockMatchRepo) ListByUserID(_ context.Context, _ string, _, _ int) ([]*aggregate.SoloMatch, error) {
	return nil, nil
}

func buildPendingReadyMatch(matchID string) *aggregate.SoloMatch {
	p1, _ := entity.NewMatchParticipant("p1", matchID, "user-1")
	p2, _ := entity.NewMatchParticipant("p2", matchID, "user-2")
	now := time.Now().UTC()
	match, _ := aggregate.NewSoloMatch(
		matchID,
		"ex-1",
		"Push-up",
		valueobject.ChallengeTypeReps,
		[]*entity.MatchParticipant{p1, p2},
		now,
	)
	return match
}

func TestSoloMatchRoomReadyConsumer_Success(t *testing.T) {
	matchID := "match-ready-123"
	match := buildPendingReadyMatch(matchID)

	repo := &roomReadyMockMatchRepo{
		matches: map[string]*aggregate.SoloMatch{
			matchID: match,
		},
	}

	c := consumer.NewSoloMatchRoomReadyConsumer(repo, nil)

	startedTime := time.Now().UTC().Add(-2 * time.Second)
	payload := map[string]interface{}{
		"specversion": "1.0",
		"id":          "evt-123",
		"source":      "/workout_execution",
		"type":        "contracts.core.workout_execution.v1.soloMatchRoomReady",
		"time":        time.Now().UTC().Format(time.RFC3339),
		"data": map[string]interface{}{
			"matchId":   matchID,
			"startedAt": startedTime.Format(time.RFC3339),
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := repo.matches[matchID]
	if updated.Status() != valueobject.MatchStatusInProgress {
		t.Fatalf("expected status %s, got %s", valueobject.MatchStatusInProgress, updated.Status())
	}
	if updated.StartedAt() == nil || updated.StartedAt().Unix() != startedTime.Unix() {
		t.Errorf("expected startedAt %v, got %v", startedTime, updated.StartedAt())
	}
}

func TestSoloMatchRoomReadyConsumer_TerminalStateRule_CancelledMatchNeverReactivated(t *testing.T) {
	matchID := "match-cancelled-456"
	match := buildPendingReadyMatch(matchID)

	// Giả lập trận đấu đã bị hủy do timeout điểm danh (quá 60s)
	if err := match.CancelOnNoShow(time.Now().UTC().Add(70 * time.Second)); err != nil {
		t.Fatalf("failed to cancel match on no show: %v", err)
	}
	if match.Status() != valueobject.MatchStatusCancelled {
		t.Fatalf("precondition failed: match status must be CANCELLED, got %s", match.Status())
	}

	repo := &roomReadyMockMatchRepo{
		matches: map[string]*aggregate.SoloMatch{
			matchID: match,
		},
	}

	c := consumer.NewSoloMatchRoomReadyConsumer(repo, nil)

	// Event phòng sẵn sàng đến muộn
	payload := map[string]interface{}{
		"specversion": "1.0",
		"id":          "evt-late-123",
		"source":      "/workout_execution",
		"type":        "contracts.core.workout_execution.v1.soloMatchRoomReady",
		"data": map[string]interface{}{
			"matchId": matchID,
		},
	}
	raw, _ := json.Marshal(payload)

	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Xác nhận trạng thái vẫn phải là CANCELLED (Terminal State Rule)
	persisted := repo.matches[matchID]
	if persisted.Status() != valueobject.MatchStatusCancelled {
		t.Fatalf("TERMINAL STATE VIOLATION: Cancelled match was reactivated to %s!", persisted.Status())
	}
}

func TestSoloMatchRoomReadyConsumer_IgnoredNonMatchingEvent(t *testing.T) {
	repo := &roomReadyMockMatchRepo{matches: make(map[string]*aggregate.SoloMatch)}
	c := consumer.NewSoloMatchRoomReadyConsumer(repo, nil)

	payload := map[string]interface{}{
		"type": "contracts.core.user.v1.userRegistered",
	}
	raw, _ := json.Marshal(payload)

	if err := c.HandleMessage(context.Background(), raw); err != nil {
		t.Fatalf("unexpected error for unrelated event: %v", err)
	}
}

type roomReadyMockOutboxLogRepo struct {
	processed map[string]bool
	saved     map[string]string // eventID -> status
	errMsg    map[string]string
}

func (m *roomReadyMockOutboxLogRepo) Save(_ context.Context, eventID, _ string, _ []byte, _ string, status, errMsg string) error {
	m.saved[eventID] = status
	m.errMsg[eventID] = errMsg
	if status == "PROCESSED" || status == "SUCCESS" {
		m.processed[eventID] = true
	}
	return nil
}

func (m *roomReadyMockOutboxLogRepo) IsProcessed(_ context.Context, eventID string) (bool, error) {
	return m.processed[eventID], nil
}

func TestSoloMatchRoomReadyConsumer_OutboxLog_SuccessAndFailed(t *testing.T) {
	ctx := context.Background()

	t.Run("records PROCESSED when match is successfully activated", func(t *testing.T) {
		matchID := "match-log-success"
		match := buildPendingReadyMatch(matchID)
		repo := &roomReadyMockMatchRepo{
			matches: map[string]*aggregate.SoloMatch{matchID: match},
		}
		outboxLog := &roomReadyMockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
			errMsg:    make(map[string]string),
		}
		c := consumer.NewSoloMatchRoomReadyConsumer(repo, outboxLog)

		evtID := "evt-succ-1"
		payload := map[string]interface{}{
			"specversion": "1.0",
			"id":          evtID,
			"source":      "/workout_execution",
			"type":        "contracts.core.workout_execution.v1.soloMatchRoomReady",
			"data": map[string]interface{}{
				"matchId": matchID,
			},
		}
		raw, _ := json.Marshal(payload)

		err := c.HandleMessage(ctx, raw)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if outboxLog.saved[evtID] != "PROCESSED" {
			t.Errorf("expected status PROCESSED, got %s", outboxLog.saved[evtID])
		}
		if outboxLog.errMsg[evtID] != "" {
			t.Errorf("expected empty errMsg, got %s", outboxLog.errMsg[evtID])
		}
	})

	t.Run("records FAILED with errMsg when repo encounters error", func(t *testing.T) {
		repo := &roomReadyMockMatchRepo{
			err: context.DeadlineExceeded,
		}
		outboxLog := &roomReadyMockOutboxLogRepo{
			processed: make(map[string]bool),
			saved:     make(map[string]string),
			errMsg:    make(map[string]string),
		}
		c := consumer.NewSoloMatchRoomReadyConsumer(repo, outboxLog)

		evtID := "evt-fail-1"
		payload := map[string]interface{}{
			"specversion": "1.0",
			"id":          evtID,
			"source":      "/workout_execution",
			"type":        "contracts.core.workout_execution.v1.soloMatchRoomReady",
			"data": map[string]interface{}{
				"matchId": "match-err",
			},
		}
		raw, _ := json.Marshal(payload)

		err := c.HandleMessage(ctx, raw)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if outboxLog.saved[evtID] != "FAILED" {
			t.Errorf("expected status FAILED, got %s", outboxLog.saved[evtID])
		}
		if outboxLog.errMsg[evtID] == "" {
			t.Errorf("expected non-empty errMsg for failed status")
		}
	})

	t.Run("skips execution when event was already processed", func(t *testing.T) {
		repo := &roomReadyMockMatchRepo{
			matches: make(map[string]*aggregate.SoloMatch),
		}
		outboxLog := &roomReadyMockOutboxLogRepo{
			processed: map[string]bool{"evt-dup-1": true},
			saved:     make(map[string]string),
			errMsg:    make(map[string]string),
		}
		c := consumer.NewSoloMatchRoomReadyConsumer(repo, outboxLog)

		payload := map[string]interface{}{
			"specversion": "1.0",
			"id":          "evt-dup-1",
			"type":        "contracts.core.workout_execution.v1.soloMatchRoomReady",
			"data": map[string]interface{}{
				"matchId": "match-never-called",
			},
		}
		raw, _ := json.Marshal(payload)

		err := c.HandleMessage(ctx, raw)
		if err != nil {
			t.Fatalf("expected nil error on idempotent skip, got %v", err)
		}
	})
}
