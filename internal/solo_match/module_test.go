package solomatch_test

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	solomatch "github.com/viethung213/gym-companion/internal/solo_match"
)

func TestSoloMatchModuleLifecycle(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	db, err := sql.Open("sqlite", "file:solomatch_module_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer db.Close()

	// Initialize tables needed by solo_match (Không còn bảng matchmaking_tickets)
	schemaSQL := `
		ATTACH DATABASE ':memory:' AS solo_match;
		CREATE TABLE IF NOT EXISTS solo_match.user_snapshots (
			user_id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			avatar_url TEXT,
			synced_at TIMESTAMP NOT NULL
		);
		CREATE TABLE IF NOT EXISTS solo_match.solo_matches (
			id TEXT PRIMARY KEY,
			exercise_id TEXT NOT NULL,
			exercise_name TEXT NOT NULL,
			challenge_type TEXT NOT NULL,
			status TEXT NOT NULL,
			winner_id TEXT,
			is_draw NUMERIC DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			ended_at TIMESTAMP,
			timeout_at TIMESTAMP NOT NULL
		);
		CREATE TABLE IF NOT EXISTS solo_match.match_participants (
			id TEXT PRIMARY KEY,
			match_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			result TEXT NOT NULL,
			score_value REAL NOT NULL,
			completed_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS solo_match.outbox (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			partition_key TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			published BOOLEAN DEFAULT 0,
			published_at TIMESTAMP,
			status TEXT NOT NULL,
			locked_until TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS solo_match.outbox_log (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			partition_key TEXT NOT NULL,
			processed_at TIMESTAMP NOT NULL,
			status TEXT NOT NULL,
			error_message TEXT
		);
		CREATE TABLE IF NOT EXISTS solo_match.system_bots (
			user_id TEXT PRIMARY KEY REFERENCES user_snapshots(user_id),
			difficulty_level TEXT,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL
		);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, shutdown, err := solomatch.Initialize(ctx, solomatch.ModuleDeps{
		DB:            db,
		RedisClient:   rdb,
		KafkaRegistry: nil,
	})
	if err != nil {
		t.Fatalf("solomatch.Initialize failed: %v", err)
	}
	if handler == nil {
		t.Fatal("expected non-nil GRPCHandler")
	}

	// Mount on HTTP mux
	mux := http.NewServeMux()
	solomatch.RegisterConnectHandler(mux, handler)

	// Test shutdown
	doneCh := make(chan struct{})
	go func() {
		shutdown()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// success
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timed out")
	}
}
