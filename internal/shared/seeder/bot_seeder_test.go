package seeder_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	authCommand "github.com/viethung213/gym-companion/internal/auth/application/command"
	authCrypto "github.com/viethung213/gym-companion/internal/auth/infrastructure/crypto"
	authEvent "github.com/viethung213/gym-companion/internal/auth/infrastructure/event"
	authPostgres "github.com/viethung213/gym-companion/internal/auth/infrastructure/persistence/postgres"
	authGRPC "github.com/viethung213/gym-companion/internal/auth/transport/grpc"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
	"gorm.io/gorm"
)

func setupTestDatabase(t *testing.T) (*sql.DB, *gorm.DB) {
	t.Helper()

	gormDB, err := gorm.Open(sqlite.Open("file:bot_seeder_test?mode=memory&cache=shared"), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("failed to open gorm sqlite: %v", err)
	}

	db, err := gormDB.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB from gorm: %v", err)
	}
	db.SetMaxOpenConns(1)

	initSQL := `
		ATTACH DATABASE ':memory:' AS auth;
		ATTACH DATABASE ':memory:' AS solo_match;

		-- Auth schema
		CREATE TABLE IF NOT EXISTS auth.roles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP NOT NULL
		);
		CREATE TABLE IF NOT EXISTS auth.users (
			id TEXT PRIMARY KEY,
			full_name TEXT,
			role_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);
		CREATE TABLE IF NOT EXISTS auth.user_identities (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			identity_type TEXT NOT NULL,
			identifier TEXT NOT NULL,
			credential_data TEXT,
			metadata TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);
		CREATE TABLE IF NOT EXISTS auth.outbox (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL UNIQUE,
			event_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			partition_key TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			published BOOLEAN DEFAULT 0,
			published_at TIMESTAMP,
			status TEXT NOT NULL,
			locked_until TIMESTAMP
		);

		-- Ensure default User roles
		INSERT OR IGNORE INTO auth.roles (id, name, description, created_at)
		VALUES ('user', 'User', 'Standard user', CURRENT_TIMESTAMP);
		INSERT OR IGNORE INTO auth.roles (id, name, description, created_at)
		VALUES ('User', 'User', 'Standard user', CURRENT_TIMESTAMP);

		-- Solo Match schema
		CREATE TABLE IF NOT EXISTS solo_match.user_snapshots (
			user_id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			avatar_url TEXT NOT NULL DEFAULT '',
			rating_score INTEGER NOT NULL DEFAULT 1000,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS solo_match.system_bots (
			user_id TEXT PRIMARY KEY,
			difficulty_level TEXT NOT NULL DEFAULT 'INTERMEDIATE',
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`

	if _, err := db.Exec(initSQL); err != nil {
		t.Fatalf("failed to initialize test schemas: %v", err)
	}

	return db, gormDB
}

func TestSeedSystemBots(t *testing.T) {
	db, gormDB := setupTestDatabase(t)
	defer db.Close()

	ctx := context.Background()

	userRepo := authPostgres.NewUserRepository(gormDB)
	outboxRepo := authPostgres.NewOutboxRepository(gormDB)
	argon2Hasher := authCrypto.NewArgon2Hasher()
	txManager := authPostgres.NewSQLTransactionManager(gormDB)
	eventPub := authEvent.NewOutboxWriter(outboxRepo)
	registerHandler := authCommand.NewRegisterHandler(userRepo, argon2Hasher, eventPub, txManager)

	// Auth API Handler (Transport layer)
	authHandler := authGRPC.NewGRPCHandler(
		nil, nil, nil, nil, nil, nil,
		registerHandler,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	// Simulate Solo Match Kafka consumer syncing user snapshots from auth.users
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				if _, err := db.Exec(`
					INSERT OR IGNORE INTO solo_match.user_snapshots (user_id, display_name, avatar_url, rating_score)
					SELECT id, full_name, '', 1000 
					FROM auth.users
				`); err != nil {
					t.Logf("mock sync worker error: %v", err)
				}
			}
		}
	}()

	// 1. First Run: Seeds all bot profiles through the AuthService API (Transport -> Application -> Domain -> Infra)
	err := seeder.SeedSystemBots(ctx, db, authHandler)
	if err != nil {
		t.Fatalf("SeedSystemBots failed on initial run: %v", err)
	}

	expectedCount := len(seeder.DefaultBotProfiles())

	// Verify Auth: Users created
	var authUsersCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM auth.users").Scan(&authUsersCount); err != nil {
		t.Fatalf("query auth.users count: %v", err)
	}
	if authUsersCount != expectedCount {
		t.Errorf("expected %d auth.users, got %d", expectedCount, authUsersCount)
	}

	// Verify Auth: Identities created
	var authIdentitiesCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM auth.user_identities").Scan(&authIdentitiesCount); err != nil {
		t.Fatalf("query auth.user_identities count: %v", err)
	}
	if authIdentitiesCount != expectedCount {
		t.Errorf("expected %d auth.user_identities, got %d", expectedCount, authIdentitiesCount)
	}

	// Verify Auth: Outbox events generated by Identity domain logic
	var authOutboxCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM auth.outbox WHERE event_type LIKE '%userRegistered%'").Scan(&authOutboxCount); err != nil {
		t.Fatalf("query auth.outbox count: %v", err)
	}
	if authOutboxCount != expectedCount {
		t.Errorf("expected %d userRegistered outbox events, got %d", expectedCount, authOutboxCount)
	}

	// Verify Solo Match: System bots created
	var systemBotsCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM solo_match.system_bots").Scan(&systemBotsCount); err != nil {
		t.Fatalf("query solo_match.system_bots count: %v", err)
	}
	if systemBotsCount != expectedCount {
		t.Errorf("expected %d solo_match.system_bots, got %d", expectedCount, systemBotsCount)
	}

	// 2. Second Run: Verifies IDEMPOTENCY (checks database, skips execution without calling API)
	err = seeder.SeedSystemBots(ctx, db, authHandler)
	if err != nil {
		t.Fatalf("SeedSystemBots failed on second run (idempotency check): %v", err)
	}

	// Verify count remains unchanged (no duplicate events or accounts created)
	var authUsersCountAfter int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM auth.users").Scan(&authUsersCountAfter); err != nil {
		t.Fatalf("query auth.users count after second run: %v", err)
	}
	if authUsersCountAfter != expectedCount {
		t.Errorf("idempotency check failed: expected %d auth.users, got %d", expectedCount, authUsersCountAfter)
	}

	var authOutboxCountAfter int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM auth.outbox WHERE event_type LIKE '%userRegistered%'").Scan(&authOutboxCountAfter); err != nil {
		t.Fatalf("query auth.outbox count after second run: %v", err)
	}
	if authOutboxCountAfter != expectedCount {
		t.Errorf("idempotency check failed: expected %d outbox events, got %d", expectedCount, authOutboxCountAfter)
	}

	var systemBotsCountAfter int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM solo_match.system_bots").Scan(&systemBotsCountAfter); err != nil {
		t.Fatalf("query solo_match.system_bots count after second run: %v", err)
	}
	if systemBotsCountAfter != expectedCount {
		t.Errorf("idempotency check failed: expected %d solo_match.system_bots, got %d", expectedCount, systemBotsCountAfter)
	}
}

func TestSeedSystemBots_SnapshotTimeout(t *testing.T) {
	db, gormDB := setupTestDatabase(t)
	defer db.Close()

	// Context with very short timeout to trigger snapshot wait cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	userRepo := authPostgres.NewUserRepository(gormDB)
	outboxRepo := authPostgres.NewOutboxRepository(gormDB)
	argon2Hasher := authCrypto.NewArgon2Hasher()
	txManager := authPostgres.NewSQLTransactionManager(gormDB)
	eventPub := authEvent.NewOutboxWriter(outboxRepo)
	registerHandler := authCommand.NewRegisterHandler(userRepo, argon2Hasher, eventPub, txManager)

	authHandler := authGRPC.NewGRPCHandler(
		nil, nil, nil, nil, nil, nil,
		registerHandler,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	// Do NOT start mock consumer to simulate missing/lagging snapshot sync
	err := seeder.SeedSystemBots(ctx, db, authHandler)
	if err == nil {
		t.Fatal("expected SeedSystemBots to return error on snapshot sync timeout/cancellation, got nil")
	}
}
