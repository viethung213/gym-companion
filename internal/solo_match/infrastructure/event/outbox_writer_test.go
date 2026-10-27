package event_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/event"
)

type mockOutboxRepository struct {
	savedRecords []*port.OutboxRecord
}

func (m *mockOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	m.savedRecords = append(m.savedRecords, record)
	return nil
}

func (m *mockOutboxRepository) SaveBatch(ctx context.Context, records []*port.OutboxRecord) error {
	m.savedRecords = append(m.savedRecords, records...)
	return nil
}

func (m *mockOutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepository) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepository) MarkAsPublished(ctx context.Context, ids []string) error {
	return nil
}

func TestOutboxWriter_PublishEvents(t *testing.T) {
	repo := &mockOutboxRepository{}
	writer := infraEvent.NewOutboxWriter(repo)
	ctx := context.Background()

	now := time.Now().UTC()
	startEvt := &domainEvent.SoloMatchStarted{
		MatchID:       "match_test",
		ExerciseID:    "ex_pushup",
		ExerciseName:  "Push Up",
		ChallengeType: "REPS",
		Player1ID:     "user_1",
		Player2ID:     "user_2",
		BotUserID:     "user_2",
		StartedAt:     now,
	}

	err := writer.PublishEvents(ctx, []any{startEvt})
	if err != nil {
		t.Fatalf("PublishEvents err: %v", err)
	}

	if len(repo.savedRecords) != 1 {
		t.Fatalf("expected 1 record saved, got %d", len(repo.savedRecords))
	}

	rec := repo.savedRecords[0]
	if rec.PartitionKey != "user_1" {
		t.Errorf("got partition key %s, want user_1", rec.PartitionKey)
	}
	if rec.EventType != startEvt.EventName() {
		t.Errorf("got event type %s, want %s", rec.EventType, startEvt.EventName())
	}
	if len(rec.Payload) == 0 {
		t.Errorf("expected non-empty payload")
	}
}
