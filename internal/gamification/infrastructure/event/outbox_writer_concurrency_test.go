package event_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/gamification/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/gamification/infrastructure/event"
)

type concurrentMockOutboxRepo struct {
	mu      sync.Mutex
	records []*port.OutboxRecord
}

func (m *concurrentMockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, record)
	return nil
}

func (m *concurrentMockOutboxRepo) ClaimBatch(
	ctx context.Context,
	limit int,
	lockDuration time.Duration,
) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *concurrentMockOutboxRepo) MarkAsPublished(ctx context.Context, ids []string) error {
	return nil
}

func (m *concurrentMockOutboxRepo) MarkAsFailed(ctx context.Context, ids []string, lastErr error) error {
	return nil
}

func (m *concurrentMockOutboxRepo) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []*port.OutboxRecord) error,
) error {
	return nil
}

func TestOutboxWriter_Concurrency(t *testing.T) {
	repo := &concurrentMockOutboxRepo{}
	writer := infraEvent.NewOutboxWriter(repo)

	concurrency := 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	ctx := context.Background()
	now := time.Now().UTC()

	for i := 0; i < concurrency; i++ {
		workerID := i
		go func() {
			defer wg.Done()
			userID := fmt.Sprintf("user-%03d", workerID)
			ev := domainEvent.XpEarned{
				UserID:   userID,
				DeltaXp:  50,
				NewXp:    int64(workerID * 50),
				NewLevel: 1,
				Source:   "WORKOUT",
				EarnedAt: now,
			}
			err := writer.WriteEvents(ctx, "user_xp", userID, []any{ev})
			if err != nil {
				t.Errorf("worker %d WriteEvents error: %v", workerID, err)
			}
		}()
	}

	wg.Wait()

	repo.mu.Lock()
	totalSaved := len(repo.records)
	repo.mu.Unlock()

	if totalSaved != concurrency {
		t.Fatalf("concurrent OutboxWriter saved %d records, want %d", totalSaved, concurrency)
	}

	// Verify each record has unique IDs and valid payload
	seenEventIDs := make(map[string]bool)
	repo.mu.Lock()
	for _, rec := range repo.records {
		if rec.EventID == "" {
			t.Errorf("empty EventID in record: %+v", rec)
		}
		if seenEventIDs[rec.EventID] {
			t.Errorf("duplicate EventID detected: %s", rec.EventID)
		}
		seenEventIDs[rec.EventID] = true
	}
	repo.mu.Unlock()
}
