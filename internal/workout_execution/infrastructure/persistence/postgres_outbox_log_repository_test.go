//go:build integration

package persistence_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	infraPostgres "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
)

func TestPostgresOutboxLogRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewPostgresOutboxLogRepository(db)
	ctx := context.Background()

	eventID := uuid.New().String()

	// 1. Fresh eventID should not be processed
	processed, err := repo.IsProcessed(ctx, eventID)
	if err != nil {
		t.Fatalf("IsProcessed failed: %v", err)
	}
	if processed {
		t.Errorf("expected false for fresh eventID, got true")
	}

	// 2. Empty eventID should return false, nil
	processedEmpty, err := repo.IsProcessed(ctx, "")
	if err != nil || processedEmpty {
		t.Errorf("expected false, nil for empty eventID, got %v, %v", processedEmpty, err)
	}

	// 3. Save nil log record should not error
	if err := repo.SaveLog(ctx, nil); err != nil {
		t.Errorf("expected nil error for nil log record, got %v", err)
	}

	// 4. SaveLog as PROCESSED
	record := &port.OutboxLogRecord{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    "WorkoutSessionStarted",
		Payload:      []byte(`{"test":true}`),
		PartitionKey: "key-1",
		Status:       "PROCESSED",
	}
	if err := repo.SaveLog(ctx, record); err != nil {
		t.Fatalf("SaveLog failed: %v", err)
	}

	// 5. IsProcessed should now be true
	processed, err = repo.IsProcessed(ctx, eventID)
	if err != nil {
		t.Fatalf("IsProcessed failed: %v", err)
	}
	if !processed {
		t.Errorf("expected true after SaveLog PROCESSED, got false")
	}

	// 6. Test on conflict / duplicate save log (idempotent upsert)
	record.Status = "PROCESSED"
	if err := repo.SaveLog(ctx, record); err != nil {
		t.Fatalf("SaveLog upsert conflict failed: %v", err)
	}

	// 7. Canceled context should return error
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := repo.IsProcessed(ctxCancel, eventID); err == nil {
		t.Error("expected error on canceled context IsProcessed")
	}
	if err := repo.SaveLog(ctxCancel, record); err == nil {
		t.Error("expected error on canceled context SaveLog")
	}
}
