package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupUserSnapshotTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:user_snapshot_test?mode=memory&cache=shared"), &gorm.Config{
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
		CREATE TABLE IF NOT EXISTS solo_match.user_snapshots (
			user_id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			avatar_url TEXT DEFAULT '',
			synced_at DATETIME NOT NULL
		);
	`
	if err := db.Exec(schemaSQL).Error; err != nil {
		t.Fatalf("create user_snapshots schema: %v", err)
	}

	return db
}

func TestGORMUserSnapshotRepository(t *testing.T) {
	db := setupUserSnapshotTestDB(t)
	repo := persistence.NewGORMUserSnapshotRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("successfully upserts and updates on conflict", func(t *testing.T) {
		snap1, err := entity.NewUserSnapshot("user_101", "Alice", "https://avatar1.png", now)
		if err != nil {
			t.Fatalf("create snapshot: %v", err)
		}

		// 1. Initial Upsert (Insert)
		err = repo.Upsert(ctx, snap1)
		if err != nil {
			t.Fatalf("initial upsert failed: %v", err)
		}

		found, err := repo.FindByUserID(ctx, "user_101")
		if err != nil || found == nil {
			t.Fatalf("expected to find user_101, got: %v, err: %v", found, err)
		}
		if found.DisplayName() != "Alice" {
			t.Errorf("DisplayName got %s, want Alice", found.DisplayName())
		}
		if found.AvatarURL() != "https://avatar1.png" {
			t.Errorf("AvatarURL got %s, want https://avatar1.png", found.AvatarURL())
		}

		// 2. Second Upsert on same user_id (Conflict -> Update)
		later := now.Add(2 * time.Hour)
		snap1Updated, _ := entity.NewUserSnapshot("user_101", "Alice Wonderland", "https://avatar_new.png", later)
		err = repo.Upsert(ctx, snap1Updated)
		if err != nil {
			t.Fatalf("second upsert (update) failed: %v", err)
		}

		updatedFound, err := repo.FindByUserID(ctx, "user_101")
		if err != nil || updatedFound == nil {
			t.Fatalf("expected to find updated user_101, got: %v, err: %v", updatedFound, err)
		}
		if updatedFound.DisplayName() != "Alice Wonderland" {
			t.Errorf("Updated DisplayName got %s, want Alice Wonderland", updatedFound.DisplayName())
		}
		if updatedFound.AvatarURL() != "https://avatar_new.png" {
			t.Errorf("Updated AvatarURL got %s, want https://avatar_new.png", updatedFound.AvatarURL())
		}
	})

	t.Run("returns nil when finding non-existent user", func(t *testing.T) {
		found, err := repo.FindByUserID(ctx, "user_ghost")
		if err != nil {
			t.Fatalf("unexpected error finding non-existent user: %v", err)
		}
		if found != nil {
			t.Errorf("expected nil for non-existent user, got %+v", found)
		}
	})

	t.Run("successfully finds batch snapshots by multiple IDs", func(t *testing.T) {
		// Empty slice input
		emptyRes, err := repo.FindByUserIDs(ctx, []string{})
		if err != nil {
			t.Fatalf("unexpected error for empty slice: %v", err)
		}
		if len(emptyRes) != 0 {
			t.Errorf("expected empty map for empty IDs, got %d items", len(emptyRes))
		}

		// Insert user_201 and user_202
		snap201, _ := entity.NewUserSnapshot("user_201", "Bob", "", now)
		snap202, _ := entity.NewUserSnapshot("user_202", "Charlie", "https://charlie.png", now)
		_ = repo.Upsert(ctx, snap201)
		_ = repo.Upsert(ctx, snap202)

		// Batch query containing existing and non-existing IDs
		queryIDs := []string{"user_201", "user_202", "user_not_exist"}
		resMap, err := repo.FindByUserIDs(ctx, queryIDs)
		if err != nil {
			t.Fatalf("FindByUserIDs failed: %v", err)
		}
		if len(resMap) != 2 {
			t.Fatalf("expected 2 users in result map, got %d", len(resMap))
		}
		if resMap["user_201"] == nil || resMap["user_201"].DisplayName() != "Bob" {
			t.Errorf("user_201 data incorrect: %+v", resMap["user_201"])
		}
		if resMap["user_202"] == nil || resMap["user_202"].DisplayName() != "Charlie" {
			t.Errorf("user_202 data incorrect: %+v", resMap["user_202"])
		}
		if _, exists := resMap["user_not_exist"]; exists {
			t.Errorf("did not expect user_not_exist in map")
		}
	})
}
