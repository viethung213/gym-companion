package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupBotTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:bot_repo_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Open sqlite err: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	_ = db.Exec("ATTACH DATABASE ':memory:' AS solo_match;").Error

	schemaSQL := `
		CREATE TABLE IF NOT EXISTS solo_match.system_bots (
			user_id TEXT PRIMARY KEY,
			difficulty_level TEXT DEFAULT 'INTERMEDIATE',
			is_active NUMERIC DEFAULT 1,
			created_at DATETIME
		);
	`
	if err := db.Exec(schemaSQL).Error; err != nil {
		t.Fatalf("setup bot test schema err: %v", err)
	}

	_ = db.Exec("DELETE FROM solo_match.system_bots;").Error
	return db
}

func TestGORMBotRepository_IsBot(t *testing.T) {
	db := setupBotTestDB(t)
	repo := persistence.NewGORMBotRepository(db)
	ctx := context.Background()

	_ = db.Exec("INSERT INTO solo_match.system_bots (user_id, is_active) VALUES ('bot_101', 1);").Error
	_ = db.Exec("INSERT INTO solo_match.system_bots (user_id, is_active) VALUES ('bot_inactive', 0);").Error

	t.Run("returns false for empty userID", func(t *testing.T) {
		isBot, err := repo.IsBot(ctx, "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if isBot {
			t.Errorf("got true, want false for empty string")
		}
	})

	t.Run("identifies active bot via DB fallback", func(t *testing.T) {
		isBot, err := repo.IsBot(ctx, "bot_101")
		if err != nil || !isBot {
			t.Errorf("expected bot_101 to be active bot, got %v, err: %v", isBot, err)
		}
	})

	t.Run("returns false for inactive bot", func(t *testing.T) {
		isBot, err := repo.IsBot(ctx, "bot_inactive")
		if err != nil || isBot {
			t.Errorf("expected bot_inactive not to be active bot, got %v, err: %v", isBot, err)
		}
	})

	t.Run("returns false for non-existent user", func(t *testing.T) {
		isBot, err := repo.IsBot(ctx, "user_human_404")
		if err != nil || isBot {
			t.Errorf("expected user_human_404 not to be bot, got %v, err: %v", isBot, err)
		}
	})

	t.Run("identifies bot via in-memory cache after GetRandomBotUserID", func(t *testing.T) {
		// Populate cache
		_, err := repo.GetRandomBotUserID(ctx)
		if err != nil {
			t.Fatalf("GetRandomBotUserID err: %v", err)
		}

		// Subsequent IsBot hits cachedIDs
		isBot, err := repo.IsBot(ctx, "bot_101")
		if err != nil || !isBot {
			t.Errorf("expected bot_101 from cache, got %v, err: %v", isBot, err)
		}

		// Unknown ID hits cache check and returns false
		isBot, err = repo.IsBot(ctx, "user_not_bot")
		if err != nil || isBot {
			t.Errorf("expected user_not_bot to be false from cache, got %v, err: %v", isBot, err)
		}
	})
}

func TestGORMBotRepository_GetRandomBotUserID(t *testing.T) {
	t.Run("returns ErrNoBotAvailable when no active bots exist", func(t *testing.T) {
		db := setupBotTestDB(t)
		repo := persistence.NewGORMBotRepository(db)
		ctx := context.Background()

		_, err := repo.GetRandomBotUserID(ctx)
		if !errors.Is(err, derror.ErrNoBotAvailable) {
			t.Fatalf("expected ErrNoBotAvailable, got: %v", err)
		}
	})

	t.Run("fetches from DB then caches until TTL expiry", func(t *testing.T) {
		db := setupBotTestDB(t)
		repo := persistence.NewGORMBotRepository(db)
		ctx := context.Background()

		_ = db.Exec("INSERT INTO solo_match.system_bots (user_id, is_active) VALUES ('bot_alpha', 1);").Error

		// Initial load from DB into memory
		id, err := repo.GetRandomBotUserID(ctx)
		if err != nil {
			t.Fatalf("GetRandomBotUserID err: %v", err)
		}
		if id != "bot_alpha" {
			t.Fatalf("got %s, want bot_alpha", id)
		}

		// Insert new bot into DB
		_ = db.Exec("INSERT INTO solo_match.system_bots (user_id, is_active) VALUES ('bot_beta', 1);").Error

		// While cache TTL is still active, only bot_alpha is returned
		for i := 0; i < 5; i++ {
			cachedID, err := repo.GetRandomBotUserID(ctx)
			if err != nil {
				t.Fatalf("cached call err: %v", err)
			}
			if cachedID != "bot_alpha" {
				t.Errorf("expected cached bot_alpha, got %s", cachedID)
			}
		}

		// Expire TTL to trigger reload from DB
		repo.SetCacheTTL(1 * time.Millisecond)
		time.Sleep(2 * time.Millisecond)

		foundBeta := false
		for i := 0; i < 30; i++ {
			reloadedID, err := repo.GetRandomBotUserID(ctx)
			if err != nil {
				t.Fatalf("reloaded call err: %v", err)
			}
			if reloadedID == "bot_beta" {
				foundBeta = true
				break
			}
		}
		if !foundBeta {
			t.Errorf("expected reloaded cache to eventually return bot_beta")
		}
	})
}
