package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/persistence/postgres"
)

func setupTestDB(t *testing.T) *sql.DB {
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

func TestPostgresDeviceRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := postgres.NewPostgresDeviceRepository(db)
	ctx := context.Background()

	dev, err := aggregate.NewDevice("dev-1", "usr-1", "fcm-token-1", vo.DeviceTypeAndroid)
	if err != nil {
		t.Fatalf("new device error: %v", err)
	}

	// 1. Test Save (Upsert)
	if err := repo.Save(ctx, dev); err != nil {
		t.Fatalf("Save device error: %v", err)
	}

	// 2. Test GetActiveDevicesByUserID
	activeDevices, err := repo.GetActiveDevicesByUserID(ctx, "usr-1")
	if err != nil {
		t.Fatalf("GetActiveDevicesByUserID error: %v", err)
	}
	if got, want := len(activeDevices), 1; got != want {
		t.Fatalf("got %d active devices, want %d", got, want)
	}
	if got, want := activeDevices[0].DeviceToken(), "fcm-token-1"; got != want {
		t.Errorf("got token %s, want %s", got, want)
	}

	// 3. Test DeactivateTokens
	if err := repo.DeactivateTokens(ctx, []string{"fcm-token-1"}); err != nil {
		t.Fatalf("DeactivateTokens error: %v", err)
	}

	activeAfter, err := repo.GetActiveDevicesByUserID(ctx, "usr-1")
	if err != nil {
		t.Fatalf("GetActiveDevicesByUserID after deactivation error: %v", err)
	}
	if got, want := len(activeAfter), 0; got != want {
		t.Errorf("got %d active devices after deactivation, want %d", got, want)
	}
}

func TestPostgresSettingRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := postgres.NewPostgresSettingRepository(db)
	ctx := context.Background()

	// 1. Get non-existing returns ErrSettingNotFound
	_, err := repo.GetByUserID(ctx, "usr-non-existing")
	if err == nil {
		t.Fatal("got nil error for non-existing setting, want error")
	}

	setting, err := aggregate.NewDefaultSetting("usr-setting-1")
	if err != nil {
		t.Fatalf("new default setting error: %v", err)
	}

	// 2. Save
	if err := repo.Save(ctx, setting); err != nil {
		t.Fatalf("Save setting error: %v", err)
	}

	// 3. GetByUserID
	saved, err := repo.GetByUserID(ctx, "usr-setting-1")
	if err != nil {
		t.Fatalf("GetByUserID error: %v", err)
	}
	if got, want := saved.UserID(), "usr-setting-1"; got != want {
		t.Errorf("got UserID %s, want %s", got, want)
	}
	if got, want := saved.EnablePush(), true; got != want {
		t.Errorf("got EnablePush %v, want %v", got, want)
	}
}

func TestPostgresNotificationRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := postgres.NewPostgresNotificationRepository(db)
	ctx := context.Background()

	item, err := aggregate.NewInAppNotification("notif-1", "usr-notif-1", "Test Title", "Test Body", map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("new in-app notification error: %v", err)
	}

	// 1. Save
	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("Save in-app notification error: %v", err)
	}

	// 2. ListByUserID
	items, total, err := repo.ListByUserID(ctx, "usr-notif-1", 10, 0)
	if err != nil {
		t.Fatalf("ListByUserID error: %v", err)
	}
	if got, want := total, int32(1); got != want {
		t.Errorf("got total %d, want %d", got, want)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("got %d items, want %d", got, want)
	}
	if got, want := items[0].Title(), "Test Title"; got != want {
		t.Errorf("got title %s, want %s", got, want)
	}

	// 3. MarkAsRead & MarkAllAsRead
	if err := repo.MarkAsRead(ctx, "usr-notif-1", item.ID()); err != nil {
		t.Fatalf("MarkAsRead error: %v", err)
	}
	if err := repo.MarkAllAsRead(ctx, "usr-notif-1"); err != nil {
		t.Fatalf("MarkAllAsRead error: %v", err)
	}
}

func TestPostgresOutboxAndLogRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	outboxRepo := postgres.NewPostgresOutboxRepository(db)
	outboxLogRepo := postgres.NewPostgresOutboxLogRepository(db)
	ctx := context.Background()

	// 1. Test Outbox Save & FetchUnpublished
	rec := &port.OutboxRecord{
		ID:           "outbox-1",
		EventID:      "550e8400-e29b-41d4-a716-446655440000",
		EventType:    "contracts.generic.notification.v1.event.NotificationSent",
		Payload:      []byte(`{"title":"hello"}`),
		PartitionKey: "usr-1",
		Published:    false,
		CreatedAt:    time.Now().UTC(),
	}

	if err := outboxRepo.Save(ctx, rec); err != nil {
		t.Fatalf("outbox Save error: %v", err)
	}

	unpub, err := outboxRepo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished error: %v", err)
	}
	if got, want := len(unpub), 1; got != want {
		t.Fatalf("got %d unpublished records, want %d", got, want)
	}

	if err := outboxRepo.MarkPublished(ctx, []string{"outbox-1"}); err != nil {
		t.Fatalf("MarkPublished error: %v", err)
	}

	// 2. Test OutboxLog LogProcessed (Idempotency)
	fresh1, err := outboxLogRepo.LogProcessed(ctx, "550e8400-e29b-41d4-a716-446655440001", "EventType", "usr-1", []byte(`{}`), "SUCCESS", "")
	if err != nil {
		t.Fatalf("LogProcessed first call error: %v", err)
	}
	if !fresh1 {
		t.Errorf("got fresh1 = false, want true for new event")
	}

	fresh2, err := outboxLogRepo.LogProcessed(ctx, "550e8400-e29b-41d4-a716-446655440001", "EventType", "usr-1", []byte(`{}`), "SUCCESS", "")
	if err != nil {
		t.Fatalf("LogProcessed duplicate call error: %v", err)
	}
	if fresh2 {
		t.Errorf("got fresh2 = true, want false for duplicate event")
	}
}

func TestTxManager(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	txMgr := postgres.NewTxManager(db)
	ctx := context.Background()

	err := txMgr.ExecTx(ctx, func(txCtx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ExecTx error: %v", err)
	}
}
