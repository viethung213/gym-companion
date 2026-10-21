package persistence_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupOutboxLogRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:outbox_log_repo_test?mode=memory&cache=shared"), &gorm.Config{
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
		CREATE TABLE IF NOT EXISTS solo_match.outbox_log (
			id TEXT PRIMARY KEY,
			event_id TEXT,
			event_type TEXT,
			payload BLOB,
			partition_key TEXT,
			processed_at DATETIME,
			status TEXT,
			error_message TEXT
		);
	`
	if err := db.Exec(schemaSQL).Error; err != nil {
		t.Fatalf("create outbox_log test schema: %v", err)
	}

	_ = db.Exec("DELETE FROM solo_match.outbox_log;").Error
	return db
}

func TestGORMOutboxLogRepository_Save(t *testing.T) {
	db := setupOutboxLogRepoTestDB(t)
	repo := persistence.NewGORMOutboxLogRepository(db)
	ctx := context.Background()

	t.Run("saves with default status SUCCESS when status is empty", func(t *testing.T) {
		err := repo.Save(ctx, "evt_def_1", "contracts.test.Event", []byte(`{}`), "k1", "", "")
		if err != nil {
			t.Fatalf("repo.Save with empty status err: %v", err)
		}

		processed, err := repo.IsProcessed(ctx, "evt_def_1")
		if err != nil {
			t.Fatalf("IsProcessed err: %v", err)
		}
		if !processed {
			t.Errorf("expected evt_def_1 to be marked as processed (SUCCESS)")
		}
	})

	t.Run("saves with custom status and error message", func(t *testing.T) {
		err := repo.Save(ctx, "evt_fail_1", "contracts.test.Event", []byte(`{}`), "k2", "FAILED", "something went wrong")
		if err != nil {
			t.Fatalf("repo.Save with custom status err: %v", err)
		}

		// Since status is FAILED, IsProcessed should return false
		processed, err := repo.IsProcessed(ctx, "evt_fail_1")
		if err != nil {
			t.Fatalf("IsProcessed err: %v", err)
		}
		if processed {
			t.Errorf("expected evt_fail_1 to NOT be processed, got true")
		}
	})

	t.Run("handles idempotent save without error", func(t *testing.T) {
		err := repo.Save(ctx, "evt_idem_1", "contracts.test.Event", []byte(`{}`), "k3", "PROCESSED", "")
		if err != nil {
			t.Fatalf("first save err: %v", err)
		}

		// Duplicate save call
		err = repo.Save(ctx, "evt_idem_1", "contracts.test.Event", []byte(`{}`), "k3", "PROCESSED", "")
		if err != nil {
			t.Fatalf("second save should succeed idempotently: %v", err)
		}
	})
}

func TestGORMOutboxLogRepository_IsProcessed(t *testing.T) {
	db := setupOutboxLogRepoTestDB(t)
	repo := persistence.NewGORMOutboxLogRepository(db)
	ctx := context.Background()

	t.Run("returns false for empty eventID", func(t *testing.T) {
		processed, err := repo.IsProcessed(ctx, "")
		if err != nil {
			t.Fatalf("unexpected error for empty string: %v", err)
		}
		if processed {
			t.Errorf("expected false for empty eventID, got true")
		}
	})

	t.Run("returns false for unrecorded eventID", func(t *testing.T) {
		processed, err := repo.IsProcessed(ctx, "evt_ghost_999")
		if err != nil {
			t.Fatalf("unexpected error for unknown eventID: %v", err)
		}
		if processed {
			t.Errorf("expected false for unknown eventID, got true")
		}
	})

	t.Run("returns true when event status is PROCESSED or SUCCESS", func(t *testing.T) {
		_ = repo.Save(ctx, "evt_succ", "test", []byte(`{}`), "k", "SUCCESS", "")
		_ = repo.Save(ctx, "evt_proc", "test", []byte(`{}`), "k", "PROCESSED", "")

		succProcessed, err := repo.IsProcessed(ctx, "evt_succ")
		if err != nil || !succProcessed {
			t.Errorf("expected evt_succ to be true, got %v (err: %v)", succProcessed, err)
		}

		procProcessed, err := repo.IsProcessed(ctx, "evt_proc")
		if err != nil || !procProcessed {
			t.Errorf("expected evt_proc to be true, got %v (err: %v)", procProcessed, err)
		}
	})
}
