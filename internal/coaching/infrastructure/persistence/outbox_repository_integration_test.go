//go:build integration

package persistence_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/shared/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func getCoachingTestDB(t *testing.T) *gorm.DB {
	sqlDB, err := database.GetRegistry().GetPool("coaching")
	if err != nil {
		t.Fatalf("Failed to initialize coaching test database: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("Failed to wrap database in gorm: %v", err)
	}

	db.Exec("CREATE SCHEMA IF NOT EXISTS coaching;")
	db.Exec(`
		CREATE TABLE IF NOT EXISTS coaching.outbox (
			id VARCHAR(255) PRIMARY KEY,
			event_id VARCHAR(255) UNIQUE NOT NULL,
			event_type VARCHAR(255) NOT NULL,
			payload JSONB NOT NULL,
			partition_key VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			published BOOLEAN DEFAULT FALSE,
			published_at TIMESTAMP WITH TIME ZONE,
			status VARCHAR(50) DEFAULT 'PENDING' NOT NULL,
			locked_until TIMESTAMP WITH TIME ZONE
		);
	`)
	db.Exec("ALTER TABLE coaching.outbox ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'PENDING' NOT NULL;")
	db.Exec("ALTER TABLE coaching.outbox ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP WITH TIME ZONE;")

	return db
}

func truncateCoachingOutbox(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE coaching.outbox CASCADE;")
}

func TestCoachingOutboxRepository_3ConcurrentNodes_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	truncateCoachingOutbox(db)
	defer truncateCoachingOutbox(db)

	repo := persistence.NewOutboxRepository(db)
	ctx := context.Background()

	const totalRecords = 30
	const batchLimit = 10

	for i := 0; i < totalRecords; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:           id,
			EventID:      id,
			EventType:    "coaching.test",
			Payload:      []byte(`{}`),
			PartitionKey: fmt.Sprintf("key-%d", i),
			CreatedAt:    time.Now(),
		}
		err := repo.Save(ctx, rec)
		require.NoError(t, err)
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

	assert.Len(t, processedIDs, totalRecords)
	for id, count := range processedIDs {
		assert.Equal(t, 1, count, "Record %s processed more than once", id)
	}
}

func TestCoachingOutboxRepository_NodeCrashFailover_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	truncateCoachingOutbox(db)
	defer truncateCoachingOutbox(db)

	repo := persistence.NewOutboxRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:           id,
			EventID:      id,
			EventType:    "coaching.test.failover",
			Payload:      []byte(`{}`),
			PartitionKey: fmt.Sprintf("key-%d", i),
			CreatedAt:    time.Now(),
		}
		err := repo.Save(ctx, rec)
		require.NoError(t, err)
	}

	// Node 1 claims with 100ms short lock
	node1Claimed, err := repo.ClaimBatch(ctx, 5, 100*time.Millisecond)
	require.NoError(t, err)
	assert.Len(t, node1Claimed, 5)

	// Node 1 CRASHES (no MarkPublished called)

	// Node 2 tries immediately -> 0 records
	node2Immediate, err := repo.ClaimBatch(ctx, 5, 5*time.Second)
	require.NoError(t, err)
	assert.Len(t, node2Immediate, 0)

	// Wait 150ms for lock expiration
	time.Sleep(150 * time.Millisecond)

	// Node 2 tries after lock expired -> claims all 5
	node2Recovered, err := repo.ClaimBatch(ctx, 5, 5*time.Second)
	require.NoError(t, err)
	assert.Len(t, node2Recovered, 5)

	recIDs := make([]string, len(node2Recovered))
	for i, r := range node2Recovered {
		recIDs[i] = r.ID
	}
	err = repo.MarkPublished(ctx, recIDs)
	require.NoError(t, err)

	unpub, err := repo.FetchUnpublished(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, unpub, 0)
}
