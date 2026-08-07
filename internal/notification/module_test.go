package notification_test

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/notification"
)

func setupModuleDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open memory sqlite db: %v", err)
	}

	queries := []string{
		`ATTACH DATABASE ':memory:' AS notification`,
		`CREATE TABLE IF NOT EXISTS notification.user_devices (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, device_token TEXT NOT NULL,
			device_type TEXT NOT NULL, is_active INTEGER DEFAULT 1,
			created_at TIMESTAMP, updated_at TIMESTAMP, last_used_at TIMESTAMP,
			CONSTRAINT user_device_token_unique UNIQUE (user_id, device_token)
		)`,
		`CREATE TABLE IF NOT EXISTS notification.user_settings (
			user_id TEXT PRIMARY KEY, enable_push INTEGER DEFAULT 1,
			enable_email INTEGER DEFAULT 1, enable_sms INTEGER DEFAULT 0,
			quiet_hours_start TEXT DEFAULT '', quiet_hours_end TEXT DEFAULT '',
			created_at TIMESTAMP, updated_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS notification.in_app_notifications (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, title TEXT NOT NULL,
			body TEXT NOT NULL, data TEXT DEFAULT '{}', is_read INTEGER DEFAULT 0,
			created_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS notification.outbox (
			id TEXT PRIMARY KEY, event_id TEXT UNIQUE, event_type TEXT,
			payload TEXT, partition_key TEXT, created_at TIMESTAMP,
			published INTEGER DEFAULT 0, published_at TIMESTAMP,
			locked_until TIMESTAMP, status TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS notification.outbox_log (
			id TEXT PRIMARY KEY, event_id TEXT UNIQUE, event_type TEXT,
			payload TEXT, partition_key TEXT, processed_at TIMESTAMP,
			status TEXT, error_message TEXT
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("failed to execute setup query (%s): %v", q, err)
		}
	}

	return db
}

func TestInitializeModule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("Initialize with nil DB returns error", func(t *testing.T) {
		t.Parallel()

		_, _, err := notification.Initialize(ctx, notification.ModuleDeps{DB: nil})
		if err == nil {
			t.Fatal("expected error for nil DB, got nil")
		}
	})

	t.Run("Initialize and RegisterConnectHandler success", func(t *testing.T) {
		t.Parallel()

		db := setupModuleDB(t)
		defer db.Close()

		grpcHandler, cleanup, err := notification.Initialize(ctx, notification.ModuleDeps{
			DB:            db,
			KafkaRegistry: nil,
		})
		if err != nil {
			t.Fatalf("unexpected Initialize error: %v", err)
		}
		if grpcHandler == nil {
			t.Fatal("got nil GRPCHandler, want non-nil")
		}
		if cleanup == nil {
			t.Fatal("got nil cleanup, want non-nil")
		}

		mux := http.NewServeMux()
		notification.RegisterConnectHandler(mux, grpcHandler)

		cleanup()
	})
}
