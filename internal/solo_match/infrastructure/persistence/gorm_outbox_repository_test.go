package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupOutboxRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:outbox_repo_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	_ = db.Exec("ATTACH DATABASE ':memory:' AS solo_match;").Error

	schemaSQL := `
		CREATE TABLE IF NOT EXISTS solo_match.outbox (
			id TEXT PRIMARY KEY,
			event_id TEXT UNIQUE,
			event_type TEXT,
			payload BLOB,
			partition_key TEXT,
			created_at DATETIME,
			published NUMERIC DEFAULT 0,
			published_at DATETIME,
			status TEXT,
			locked_until DATETIME
		);
	`
	if err := db.Exec(schemaSQL).Error; err != nil {
		t.Fatalf("create outbox test schema: %v", err)
	}

	_ = db.Exec("DELETE FROM solo_match.outbox;").Error
	return db
}

func TestGORMOutboxRepository_Save(t *testing.T) {
	db := setupOutboxRepoTestDB(t)
	repo := persistence.NewGORMOutboxRepository(db)
	ctx := context.Background()

	t.Run("returns error when record is nil", func(t *testing.T) {
		err := repo.Save(ctx, nil)
		if err == nil {
			t.Errorf("expected error for nil record, got nil")
		}
	})

	t.Run("saves record and auto-generates id and event_id if empty", func(t *testing.T) {
		rec := &port.OutboxRecord{
			EventType:    "contracts.match.started",
			Payload:      []byte(`{"matchId":"m_1"}`),
			PartitionKey: "u_1",
		}
		err := repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("repo.Save failed: %v", err)
		}

		unpub, err := repo.FetchUnpublished(ctx, 10)
		if err != nil {
			t.Fatalf("fetch unpublished: %v", err)
		}
		if len(unpub) != 1 {
			t.Fatalf("expected 1 record, got %d", len(unpub))
		}
		if unpub[0].ID == "" || unpub[0].EventID == "" {
			t.Errorf("expected generated IDs, got ID: %s, EventID: %s", unpub[0].ID, unpub[0].EventID)
		}
	})

	t.Run("handles conflict on event_id idempotently without error", func(t *testing.T) {
		rec := &port.OutboxRecord{
			ID:           "rec_uniq",
			EventID:      "evt_uniq_123",
			EventType:    "contracts.match.started",
			Payload:      []byte(`{"matchId":"m_1"}`),
			PartitionKey: "u_1",
		}
		err := repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("first save failed: %v", err)
		}

		// Re-save with same EventID
		err = repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("duplicate save should do nothing without error, got: %v", err)
		}
	})
}

func TestGORMOutboxRepository_SaveBatch(t *testing.T) {
	db := setupOutboxRepoTestDB(t)
	repo := persistence.NewGORMOutboxRepository(db)
	ctx := context.Background()

	t.Run("returns nil for empty slice", func(t *testing.T) {
		err := repo.SaveBatch(ctx, nil)
		if err != nil {
			t.Fatalf("SaveBatch(nil) err: %v", err)
		}
		err = repo.SaveBatch(ctx, []*port.OutboxRecord{})
		if err != nil {
			t.Fatalf("SaveBatch([]) err: %v", err)
		}
	})

	t.Run("saves batch of records with auto-generated IDs", func(t *testing.T) {
		batch := []*port.OutboxRecord{
			{ID: "batch_1", EventID: "evt_b_1", EventType: "test.evt1", Payload: []byte(`{}`), PartitionKey: "k1"},
			{EventType: "test.evt2", Payload: []byte(`{}`), PartitionKey: "k2"}, // empty IDs
		}
		err := repo.SaveBatch(ctx, batch)
		if err != nil {
			t.Fatalf("SaveBatch failed: %v", err)
		}

		unpub, err := repo.FetchUnpublished(ctx, 10)
		if err != nil {
			t.Fatalf("fetch unpublished: %v", err)
		}
		if len(unpub) != 2 {
			t.Fatalf("expected 2 records, got %d", len(unpub))
		}
	})
}

func TestGORMOutboxRepository_FetchUnpublished(t *testing.T) {
	db := setupOutboxRepoTestDB(t)
	repo := persistence.NewGORMOutboxRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, &port.OutboxRecord{ID: "r1", EventID: "e1", EventType: "test.evt", Payload: []byte(`{}`), PartitionKey: "k1"})
	_ = repo.Save(ctx, &port.OutboxRecord{ID: "r2", EventID: "e2", EventType: "test.evt", Payload: []byte(`{}`), PartitionKey: "k2"})

	t.Run("defaults limit to 100 when non-positive", func(t *testing.T) {
		unpub, err := repo.FetchUnpublished(ctx, 0)
		if err != nil {
			t.Fatalf("fetch unpublished: %v", err)
		}
		if len(unpub) != 2 {
			t.Errorf("got %d unpublished, want 2", len(unpub))
		}
	})

	t.Run("respects positive limit", func(t *testing.T) {
		unpub, err := repo.FetchUnpublished(ctx, 1)
		if err != nil {
			t.Fatalf("fetch unpublished: %v", err)
		}
		if len(unpub) != 1 {
			t.Errorf("got %d unpublished, want 1", len(unpub))
		}
	})
}

func TestGORMOutboxRepository_ClaimBatch(t *testing.T) {
	db := setupOutboxRepoTestDB(t)
	repo := persistence.NewGORMOutboxRepository(db)
	ctx := context.Background()

	rec1 := &port.OutboxRecord{ID: "claim_1", EventID: "evt_c_1", EventType: "test.evt", Payload: []byte(`{}`), PartitionKey: "k1"}
	rec2 := &port.OutboxRecord{ID: "claim_2", EventID: "evt_c_2", EventType: "test.evt", Payload: []byte(`{}`), PartitionKey: "k2"}
	_ = repo.Save(ctx, rec1)
	_ = repo.Save(ctx, rec2)

	t.Run("claims available records and updates status to PROCESSING", func(t *testing.T) {
		claimed, err := repo.ClaimBatch(ctx, 10, 5*time.Second)
		if err != nil {
			t.Fatalf("ClaimBatch err: %v", err)
		}
		if len(claimed) != 2 {
			t.Fatalf("got %d claimed, want 2", len(claimed))
		}

		// Subsequent claim while lock is active should return 0 records
		secondClaim, err := repo.ClaimBatch(ctx, 10, 5*time.Second)
		if err != nil {
			t.Fatalf("second claim err: %v", err)
		}
		if len(secondClaim) != 0 {
			t.Errorf("expected 0 claimed during active lock, got %d", len(secondClaim))
		}
	})

	t.Run("re-claims records after locked_until expires", func(t *testing.T) {
		// Set locked_until in the past
		past := time.Now().UTC().Add(-10 * time.Minute)
		_ = db.Exec("UPDATE solo_match.outbox SET locked_until = ? WHERE id = 'claim_1'", past).Error

		reclaimed, err := repo.ClaimBatch(ctx, 1, 5*time.Second)
		if err != nil {
			t.Fatalf("reclaim err: %v", err)
		}
		if len(reclaimed) != 1 || reclaimed[0].ID != "claim_1" {
			t.Fatalf("expected claim_1 to be reclaimed, got: %v", reclaimed)
		}
	})

	t.Run("handles default limit and default lockDuration", func(t *testing.T) {
		// Passing 0 limit and 0 duration uses defaults (100 limit, 30s duration)
		_, err := repo.ClaimBatch(ctx, 0, 0)
		if err != nil {
			t.Fatalf("ClaimBatch with defaults failed: %v", err)
		}
	})
}

func TestGORMOutboxRepository_MarkAsPublished(t *testing.T) {
	db := setupOutboxRepoTestDB(t)
	repo := persistence.NewGORMOutboxRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, &port.OutboxRecord{ID: "pub_1", EventID: "e_pub_1", EventType: "test.evt", Payload: []byte(`{}`), PartitionKey: "k1"})

	t.Run("returns nil for empty IDs", func(t *testing.T) {
		err := repo.MarkAsPublished(ctx, []string{})
		if err != nil {
			t.Fatalf("MarkAsPublished([]) err: %v", err)
		}
	})

	t.Run("marks records as published", func(t *testing.T) {
		err := repo.MarkAsPublished(ctx, []string{"pub_1"})
		if err != nil {
			t.Fatalf("MarkAsPublished failed: %v", err)
		}

		unpub, err := repo.FetchUnpublished(ctx, 10)
		if err != nil {
			t.Fatalf("fetch unpublished err: %v", err)
		}
		if len(unpub) != 0 {
			t.Errorf("expected 0 unpublished after publish, got %d", len(unpub))
		}
	})
}
