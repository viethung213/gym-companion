package publisher_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/publisher"
	"gorm.io/gorm"
)

func TestOutboxPublisher_ProcessPendingEvents(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:publisher_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB err: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.Exec("ATTACH DATABASE ':memory:' AS solo_match;").Error; err != nil {
		t.Fatalf("attach database: %v", err)
	}

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
		t.Fatalf("create outbox schema: %v", err)
	}

	// Insert a pending event
	now := time.Now().UTC()
	event := persistence.OutboxModel{
		ID:           "evt_1",
		EventID:      "evt_1_uuid",
		EventType:    "contracts.core.solo_match.v1.soloMatchCompleted",
		Payload:      []byte(`{"specversion":"1.0","id":"evt_1_uuid","data":{"matchId":"match_1"}}`),
		PartitionKey: "user_1",
		Status:       "PENDING",
		Published:    false,
		CreatedAt:    now,
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatalf("create event: %v", err)
	}

	outboxRepo := persistence.NewGORMOutboxRepository(db)
	pub := publisher.NewOutboxPublisher(outboxRepo, nil, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go pub.Start(ctx)

	// Wait for processing
	time.Sleep(300 * time.Millisecond)
	pub.Stop()

	var updated persistence.OutboxModel
	if err := db.First(&updated, "id = ?", "evt_1").Error; err != nil {
		t.Fatalf("find event: %v", err)
	}

	if updated.Status != "PUBLISHED" {
		t.Errorf("got status %s, want PUBLISHED", updated.Status)
	}
	if !updated.Published {
		t.Errorf("got published false, want true")
	}
}
