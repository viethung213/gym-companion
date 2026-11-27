package persistence_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/persistence"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*sql.DB, *gorm.DB) {
	t.Helper()

	gormDB, err := gorm.Open(sqlite.Open("file:notification_test?mode=memory&cache=shared"), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

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
			enable_email INTEGER DEFAULT 1, enable_sms INTEGER DEFAULT 1,
			quiet_hours_start TEXT DEFAULT '', quiet_hours_end TEXT DEFAULT '',
			created_at TIMESTAMP, updated_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS notification.in_app_notifications (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, title TEXT NOT NULL,
			body TEXT NOT NULL, data TEXT DEFAULT '{}', is_read INTEGER DEFAULT 0,
			created_at TIMESTAMP, event_id TEXT,
			CONSTRAINT uq_notification_in_app_user_event UNIQUE (user_id, event_id)
		)`,
		`CREATE TABLE IF NOT EXISTS notification.outbox (
			id TEXT PRIMARY KEY, event_id TEXT UNIQUE, aggregate_type TEXT, aggregate_id TEXT, event_type TEXT,
			payload TEXT, partition_key TEXT, created_at TIMESTAMP,
			published INTEGER DEFAULT 0, published_at TIMESTAMP,
			locked_until TIMESTAMP, status TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS notification.outbox_log (
			id TEXT PRIMARY KEY, event_id TEXT UNIQUE, event_type TEXT,
			payload TEXT, partition_key TEXT, processed_at TIMESTAMP,
			status TEXT, error_message TEXT, retry_count INTEGER DEFAULT 0, next_retry_at TIMESTAMP
		)`,
	}

	for _, q := range queries {
		if _, err := sqlDB.Exec(q); err != nil {
			t.Fatalf("failed to execute setup query (%s): %v", q, err)
		}
	}

	return sqlDB, gormDB
}

func TestPostgresDeviceRepository(t *testing.T) {
	sqlDB, db := setupTestDB(t)
	defer sqlDB.Close()

	repo := persistence.NewDeviceRepository(db)
	ctx := context.Background()

	dev, err := aggregate.NewDevice("dev-1", "usr-1", "fcm-token-1", vo.DeviceTypeAndroid)
	if err != nil {
		t.Fatalf("new device error: %v", err)
	}

	// 1. Test Save (Upsert)
	if err := repo.Save(ctx, dev); err != nil {
		t.Fatalf("Save device error: %v", err)
	}

	// 2. Test GetActiveDevicesByUserID & GetAllActiveUserIDs
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

	userIDs, err := repo.GetAllActiveUserIDs(ctx)
	if err != nil {
		t.Fatalf("GetAllActiveUserIDs error: %v", err)
	}
	if got, want := len(userIDs), 1; got != want {
		t.Fatalf("got %d active users, want %d", got, want)
	}
	if userIDs[0] != "usr-1" {
		t.Errorf("got user_id %s, want usr-1", userIDs[0])
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
	sqlDB, db := setupTestDB(t)
	defer sqlDB.Close()

	repo := persistence.NewSettingRepository(db)
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
	sqlDB, db := setupTestDB(t)
	defer sqlDB.Close()

	repo := persistence.NewNotificationRepository(db)
	ctx := context.Background()

	item, err := aggregate.NewInAppNotification("notif-1", "usr-notif-1", "Test Title", "Test Body", map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("new in-app notification error: %v", err)
	}
	item.WithEventID("event-test-123")

	// 1. Save
	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("Save in-app notification error: %v", err)
	}

	// 1b. Test FindByUserAndEventID
	found, err := repo.FindByUserAndEventID(ctx, "usr-notif-1", "event-test-123")
	if err != nil {
		t.Fatalf("FindByUserAndEventID error: %v", err)
	}
	if found == nil || found.ID() != item.ID() {
		t.Fatalf("expected found notification %s, got %v", item.ID(), found)
	}
	if found.EventID() != "event-test-123" {
		t.Errorf("expected EventID event-test-123, got %s", found.EventID())
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
	sqlDB, db := setupTestDB(t)
	defer sqlDB.Close()

	outboxRepo := persistence.NewOutboxRepository(db)
	outboxLogRepo := persistence.NewOutboxLogRepository(db)
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
	fresh1, err := outboxLogRepo.LogProcessed(ctx, "550e8400-e29b-41d4-a716-446655440001", "EventType", "usr-1", []byte(`{}`), "PROCESSING", "")
	if err != nil {
		t.Fatalf("LogProcessed first call error: %v", err)
	}
	if !fresh1 {
		t.Errorf("got fresh1 = false, want true for new event")
	}

	fresh2, err := outboxLogRepo.LogProcessed(ctx, "550e8400-e29b-41d4-a716-446655440001", "EventType", "usr-1", []byte(`{}`), "PROCESSING", "")
	if err != nil {
		t.Fatalf("LogProcessed second call error: %v", err)
	}
	if fresh2 {
		t.Errorf("got fresh2 = true, want false for duplicate event_id")
	}

	// 2.1 Test OutboxLog LogProcessed allows retry when previous attempt was FAILED
	failErr := outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
		EventID:      "550e8400-e29b-41d4-a716-446655440001",
		EventType:    "EventType",
		Payload:      []byte(`{}`),
		PartitionKey: "usr-1",
		Status:       "FAILED",
		ErrorMessage: "network timeout",
	})
	if failErr != nil {
		t.Fatalf("SaveLog FAILED error: %v", failErr)
	}

	freshRetry, err := outboxLogRepo.LogProcessed(ctx, "550e8400-e29b-41d4-a716-446655440001", "EventType", "usr-1", []byte(`{}`), "PROCESSING", "")
	if err != nil {
		t.Fatalf("LogProcessed retry call error: %v", err)
	}
	if !freshRetry {
		t.Errorf("got freshRetry = false, want true when previous attempt was FAILED")
	}

	// 3. Test OutboxLog SaveLog updates existing record to SUCCESS
	err = outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
		EventID:      "550e8400-e29b-41d4-a716-446655440001",
		EventType:    "EventType",
		Payload:      []byte(`{}`),
		PartitionKey: "usr-1",
		Status:       "SUCCESS",
		ErrorMessage: "",
	})
	if err != nil {
		t.Fatalf("SaveLog error: %v", err)
	}

	var logs []persistence.OutboxLogModel
	if err := db.Where("event_id = ?", "550e8400-e29b-41d4-a716-446655440001").Find(&logs).Error; err != nil {
		t.Fatalf("query outbox_log error: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("got %d logs, want 1", len(logs))
	} else if logs[0].Status != "SUCCESS" {
		t.Errorf("got status %s, want SUCCESS", logs[0].Status)
	}
}

func TestPostgresOutboxLog_ConcurrentRetryRace_MutualExclusion(t *testing.T) {
	sqlDB, db := setupTestDB(t)
	defer sqlDB.Close()

	outboxLogRepo := persistence.NewOutboxLogRepository(db)
	ctx := context.Background()

	eventID := "550e8400-e29b-41d4-a716-446655440099"

	// Initial failed state
	err := outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
		EventID:      eventID,
		EventType:    "TestEvent",
		Payload:      []byte(`{}`),
		PartitionKey: "usr-race",
		Status:       "FAILED",
		ErrorMessage: "push failed",
	})
	if err != nil {
		t.Fatalf("SaveLog initial FAILED error: %v", err)
	}

	// Simulate concurrent race: 5 workers (e.g. Kafka Consumer + InboundEventRetryWorker nodes)
	// simultaneously trying to process the failed event via LogProcessed
	const workers = 5
	results := make(chan bool, workers)

	for i := 0; i < workers; i++ {
		go func() {
			fresh, pErr := outboxLogRepo.LogProcessed(ctx, eventID, "TestEvent", "usr-race", []byte(`{}`), "PROCESSING", "")
			if pErr != nil {
				results <- false
				return
			}
			results <- fresh
		}()
	}

	trueCount := 0
	falseCount := 0
	for i := 0; i < workers; i++ {
		if <-results {
			trueCount++
		} else {
			falseCount++
		}
	}

	// Exactly ONE worker must win the race and transition FAILED -> PROCESSING
	if trueCount != 1 {
		t.Errorf("got %d workers with fresh=true, want exactly 1 (race condition!)", trueCount)
	}
	if falseCount != workers-1 {
		t.Errorf("got %d workers with fresh=false, want %d", falseCount, workers-1)
	}
}

func TestPostgresOutboxLog_ZombieProcessingTimeout(t *testing.T) {
	sqlDB, db := setupTestDB(t)
	defer sqlDB.Close()

	outboxLogRepo := persistence.NewOutboxLogRepository(db)
	ctx := context.Background()

	eventID := "550e8400-e29b-41d4-a716-446655440088"

	// 1. Simulate server crash: Record stuck in PROCESSING with processed_at = 10 minutes ago
	staleProcessedAt := time.Now().UTC().Add(-10 * time.Minute)
	err := db.Exec(`
		INSERT INTO notification.outbox_log (id, event_id, event_type, payload, partition_key, processed_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "log-zombie-1", eventID, "TestEvent", "{}", "usr-zombie", staleProcessedAt, "PROCESSING").Error
	if err != nil {
		t.Fatalf("insert stale PROCESSING record error: %v", err)
	}

	// 2. Worker restarts or Kafka redelivers message: LogProcessed should reclaim zombie lock (fresh = true)
	fresh, err := outboxLogRepo.LogProcessed(ctx, eventID, "TestEvent", "usr-zombie", []byte(`{}`), "PROCESSING", "")
	if err != nil {
		t.Fatalf("LogProcessed zombie recovery error: %v", err)
	}
	if !fresh {
		t.Errorf("got fresh = false, want true when reclaiming stale PROCESSING lock (> 5m)")
	}

	// Verify that processed_at has been renewed to recent time
	var updated persistence.OutboxLogModel
	if err := db.Where("event_id = ?", eventID).First(&updated).Error; err != nil {
		t.Fatalf("query updated outbox_log error: %v", err)
	}
	if updated.ProcessedAt.Before(time.Now().UTC().Add(-1 * time.Minute)) {
		t.Errorf("expected renewed processed_at, got %v", updated.ProcessedAt)
	}

	// 3. Immediate subsequent call within 5m must NOT be reclaimed (must be treated as duplicate / in-progress)
	freshDup, err := outboxLogRepo.LogProcessed(ctx, eventID, "TestEvent", "usr-zombie", []byte(`{}`), "PROCESSING", "")
	if err != nil {
		t.Fatalf("LogProcessed duplicate error: %v", err)
	}
	if freshDup {
		t.Errorf("got freshDup = true, want false when PROCESSING is still active (< 5m)")
	}
}

func TestPostgresOutboxLog_ConcurrentZombieReclaim_MutualExclusion(t *testing.T) {
	sqlDB, db := setupTestDB(t)
	defer sqlDB.Close()

	outboxLogRepo := persistence.NewOutboxLogRepository(db)
	ctx := context.Background()

	eventID := "550e8400-e29b-41d4-a716-446655440077"

	// Initial zombie state: PROCESSING 15 minutes ago
	staleTime := time.Now().UTC().Add(-15 * time.Minute)
	err := db.Exec(`
		INSERT INTO notification.outbox_log (id, event_id, event_type, payload, partition_key, processed_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "log-zombie-race-1", eventID, "TestEvent", "{}", "usr-race", staleTime, "PROCESSING").Error
	if err != nil {
		t.Fatalf("insert stale PROCESSING record error: %v", err)
	}

	// Simulate 5 concurrent workers trying to reclaim the same zombie lock
	const workers = 5
	results := make(chan bool, workers)

	for i := 0; i < workers; i++ {
		go func() {
			fresh, pErr := outboxLogRepo.LogProcessed(ctx, eventID, "TestEvent", "usr-race", []byte(`{}`), "PROCESSING", "")
			if pErr != nil {
				results <- false
				return
			}
			results <- fresh
		}()
	}

	trueCount := 0
	falseCount := 0
	for i := 0; i < workers; i++ {
		if <-results {
			trueCount++
		} else {
			falseCount++
		}
	}

	// Exactly ONE worker must succeed in claiming the zombie lock
	if trueCount != 1 {
		t.Errorf("got %d workers with fresh=true, want exactly 1 for zombie reclaim race", trueCount)
	}
	if falseCount != workers-1 {
		t.Errorf("got %d workers with fresh=false, want %d", falseCount, workers-1)
	}
}

func TestTxManager(t *testing.T) {
	sqlDB, db := setupTestDB(t)
	defer sqlDB.Close()

	txMgr := persistence.NewTxManager(db)
	ctx := context.Background()

	err := txMgr.ExecTx(ctx, func(txCtx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ExecTx error: %v", err)
	}

	err = txMgr.WithTransaction(ctx, func(txCtx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction error: %v", err)
	}
}
