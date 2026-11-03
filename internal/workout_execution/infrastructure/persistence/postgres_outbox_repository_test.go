//go:build integration

package persistence_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	infraPostgres "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
)

func TestPostgresOutboxRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	evID := uuid.NewString()
	record := &port.OutboxRecord{
		ID:            evID,
		AggregateType: "WorkoutSession",
		AggregateID:   "sess-1",
		EventType:     "WorkoutSessionStarted",
		Payload:       []byte(`{"test":true}`),
		Published:     false,
		CreatedAt:     time.Now().UTC(),
	}

	if err := repo.Save(ctx, record); err != nil {
		t.Fatalf("Failed to save OutboxRecord: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil || len(unpublished) != 1 {
		t.Fatalf("FetchUnpublished failed: err=%v, len=%d", err, len(unpublished))
	}

	// MarkPublished with empty slice (len==0)
	if err := repo.MarkPublished(ctx, []string{}); err != nil {
		t.Fatalf("MarkPublished empty slice failed: %v", err)
	}

	if err := repo.MarkPublished(ctx, []string{evID}); err != nil {
		t.Fatalf("MarkPublished failed: %v", err)
	}

	// ProcessBatch test
	err = repo.ProcessBatch(ctx, 10, func(txCtx context.Context, records []*port.OutboxRecord) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}
}

func TestWorkoutExecutionOutboxRepository_CanceledContext(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()

	outboxRepo := infraPostgres.NewPostgresOutboxRepository(db)

	outboxRecord := &port.OutboxRecord{ID: "o1", EventType: "ev1", Payload: []byte("{}")}
	if err := outboxRepo.Save(ctxCancel, outboxRecord); err == nil {
		t.Error("expected error on canceled context Save Outbox")
	}
	if _, err := outboxRepo.FetchUnpublished(ctxCancel, 10); err == nil {
		t.Error("expected error on canceled context FetchUnpublished")
	}
	if err := outboxRepo.MarkPublished(ctxCancel, []string{"o1"}); err == nil {
		t.Error("expected error on canceled context MarkPublished")
	}

	err := outboxRepo.ProcessBatch(context.Background(), 10, func(txCtx context.Context, records []*port.OutboxRecord) error {
		return nil
	})
	if err != nil {
		t.Errorf("ProcessBatch returned unexpected error: %v", err)
	}
}

func TestWorkoutExecutionOutboxRepository_3ConcurrentNodes_Integration(t *testing.T) {
	db := getTestDB(t)
	ensureTablesExist(db)

	repo := infraPostgres.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	const totalRecords = 30
	const batchLimit = 10

	for i := 0; i < totalRecords; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:            id,
			EventID:       id,
			AggregateType: "WorkoutSession",
			AggregateID:   fmt.Sprintf("sess-%d", i),
			EventType:     "WorkoutSessionStarted",
			Payload:       []byte(`{}`),
			PartitionKey:  fmt.Sprintf("key-%d", i),
			CreatedAt:     time.Now(),
		}
		err := repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("Failed to seed record %d: %v", i, err)
		}
	}

	var mu sync.Mutex
	processedIDs := make(map[string]int)

	var wg sync.WaitGroup
	const nodeCount = 3
	wg.Add(nodeCount)

	for nodeIdx := 1; nodeIdx <= nodeCount; nodeIdx++ {
		go func(id int) {
			defer wg.Done()
			for {
				records, err := repo.ClaimBatch(ctx, batchLimit, 5*time.Second)
				if err != nil || len(records) == 0 {
					break
				}

				mu.Lock()
				recIDs := make([]string, len(records))
				for i, r := range records {
					processedIDs[r.ID]++
					recIDs[i] = r.ID
				}
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
				_ = repo.MarkPublished(ctx, recIDs)
			}
		}(nodeIdx)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(processedIDs) != totalRecords {
		t.Errorf("Expected %d total unique records processed across 3 nodes, got %d", totalRecords, len(processedIDs))
	}

	for id, count := range processedIDs {
		if count > 1 {
			t.Errorf("Record %s was processed %d times (expected exactly 1)", id, count)
		}
	}
}

func TestWorkoutExecutionOutboxRepository_NodeCrashFailover_Integration(t *testing.T) {
	db := getTestDB(t)
	ensureTablesExist(db)

	repo := infraPostgres.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:            id,
			EventID:       id,
			AggregateType: "WorkoutSession",
			AggregateID:   fmt.Sprintf("sess-%d", i),
			EventType:     "WorkoutSessionStarted",
			Payload:       []byte(`{}`),
			PartitionKey:  fmt.Sprintf("key-%d", i),
			CreatedAt:     time.Now(),
		}
		err := repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("Failed to seed record %d: %v", i, err)
		}
	}

	// Node 1 claims with 100ms lock
	node1Claimed, err := repo.ClaimBatch(ctx, 5, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Node 1 ClaimBatch failed: %v", err)
	}
	if len(node1Claimed) != 5 {
		t.Fatalf("Node 1 expected 5 claimed records, got %d", len(node1Claimed))
	}

	// Node 1 CRASHES (no MarkPublished)

	// Node 2 tries immediately -> 0 records
	node2Immediate, err := repo.ClaimBatch(ctx, 5, 5*time.Second)
	if err != nil {
		t.Fatalf("Node 2 immediate ClaimBatch failed: %v", err)
	}
	if len(node2Immediate) != 0 {
		t.Errorf("Node 2 expected 0 records while Node 1 lock active, got %d", len(node2Immediate))
	}

	// Wait 150ms for lock expiration
	time.Sleep(150 * time.Millisecond)

	// Node 2 tries after lock expired -> claims all 5
	node2Recovered, err := repo.ClaimBatch(ctx, 5, 5*time.Second)
	if err != nil {
		t.Fatalf("Node 2 failover ClaimBatch failed: %v", err)
	}
	if len(node2Recovered) != 5 {
		t.Errorf("Node 2 expected 5 recovered records after Node 1 crash, got %d", len(node2Recovered))
	}

	recIDs := make([]string, len(node2Recovered))
	for i, r := range node2Recovered {
		recIDs[i] = r.ID
	}
	if err := repo.MarkPublished(ctx, recIDs); err != nil {
		t.Fatalf("Node 2 MarkPublished failed: %v", err)
	}

	unpub, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished failed: %v", err)
	}
	if len(unpub) != 0 {
		t.Errorf("Expected 0 leftover unpublished records, got %d", len(unpub))
	}
}
