//go:build integration

package persistence

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"github.com/viethung213/gym-companion/internal/shared/cloudevent"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCoachingPersistenceAtomicityAndIdempotency(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL or DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	assertCoachingMigrationsApplied(t, db)

	roadmaps := NewPostgresRoadmapRepository(db)
	schedules := NewPostgresWeeklyScheduleRepository(db)
	inbox := NewPostgresInboxRepository(db)
	unitOfWork := NewUnitOfWork(db)
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)

	roadmap, schedule, event := persistenceFixtures(t, now, uuid.NewString())
	t.Cleanup(func() {
		deleteFixtureRows(t, db, roadmap.ID(), schedule.ID(), event.ID, "")
	})

	errRollback := errors.New("force rollback")
	err = unitOfWork.WithinTransaction(
		context.Background(),
		func(ctx context.Context) error {
			if saveErr := roadmaps.Save(ctx, roadmap, event); saveErr != nil {
				return saveErr
			}
			if saveErr := schedules.Save(ctx, schedule, nil); saveErr != nil {
				return saveErr
			}
			return errRollback
		},
	)
	if !errors.Is(err, errRollback) {
		t.Fatalf("transaction error got = %v, want rollback error", err)
	}
	assertRowCount(t, db, "coaching.workout_roadmaps", "id", roadmap.ID(), 0)
	assertRowCount(t, db, "coaching.weekly_schedules", "id", schedule.ID(), 0)
	assertRowCount(t, db, "coaching.outbox", "event_id", event.ID, 0)

	if err := roadmaps.Save(context.Background(), roadmap, event); err != nil {
		t.Fatalf("save roadmap: %v", err)
	}
	assertRowCount(t, db, "coaching.outbox", "event_id", event.ID, 1)
	var envelopeJSON string
	if err := db.Raw(
		`SELECT payload FROM coaching.outbox WHERE event_id = ?`,
		event.ID,
	).Scan(&envelopeJSON).Error; err != nil {
		t.Fatalf("load outbox envelope: %v", err)
	}
	envelope, err := cloudevent.Decode([]byte(envelopeJSON))
	if err != nil {
		t.Fatalf("decode outbox envelope: %v", err)
	}
	if envelope.Type != domain.EventTypeRoadmapInitiated {
		t.Errorf(
			"event type got = %q, want = %q",
			envelope.Type,
			domain.EventTypeRoadmapInitiated,
		)
	}

	testConcurrentActiveRoadmapInvariant(t, db, now)

	inboxEventID := "profile-event-" + uuid.NewString()
	t.Cleanup(func() {
		deleteFixtureRows(t, db, "", "", "", inboxEventID)
	})
	payload := []byte(`{"specversion":"1.0","data":{}}`)
	if err := inbox.MarkProcessed(
		context.Background(),
		inboxEventID,
		"profileCompleted",
		payload,
		roadmap.UserID(),
	); err != nil {
		t.Fatalf("mark inbox processed: %v", err)
	}
	if err := inbox.MarkProcessed(
		context.Background(),
		inboxEventID,
		"profileCompleted",
		payload,
		roadmap.UserID(),
	); err == nil {
		t.Fatal("duplicate inbox event unexpectedly succeeded")
	}
}

func testConcurrentActiveRoadmapInvariant(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()

	userID := "integration-user-" + uuid.NewString()
	first, err := domain.Initiate(uuid.NewString(), userID, now, now)
	if err != nil {
		t.Fatalf("create first concurrent roadmap: %v", err)
	}
	second, err := domain.Initiate(uuid.NewString(), userID, now, now)
	if err != nil {
		t.Fatalf("create second concurrent roadmap: %v", err)
	}
	t.Cleanup(func() {
		deleteFixtureRows(t, db, first.ID(), "", "", "")
		deleteFixtureRows(t, db, second.ID(), "", "", "")
	})

	repository := NewPostgresRoadmapRepository(db)
	unitOfWork := NewUnitOfWork(db)
	ready := sync.WaitGroup{}
	ready.Add(2)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, roadmap := range []*domain.WorkoutRoadmap{first, second} {
		go func(current *domain.WorkoutRoadmap) {
			results <- unitOfWork.WithinTransaction(
				context.Background(),
				func(ctx context.Context) error {
					ready.Done()
					<-start
					return repository.Save(ctx, current, nil)
				},
			)
		}(roadmap)
	}
	ready.Wait()
	close(start)

	succeeded := 0
	conflicted := 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, domain.ErrRoadmapAlreadyActive):
			conflicted++
		default:
			t.Fatalf("concurrent save returned unexpected error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf(
			"concurrent results succeeded=%d conflicted=%d, want 1 and 1",
			succeeded,
			conflicted,
		)
	}
}

func persistenceFixtures(
	t *testing.T,
	now time.Time,
	userID string,
) (*domain.WorkoutRoadmap, *domain.WeeklySchedule, *domain.Event) {
	t.Helper()

	roadmap, err := domain.Initiate(uuid.NewString(), userID, now, now)
	if err != nil {
		t.Fatalf("create roadmap: %v", err)
	}
	schedule, err := domain.GenerateWeeklySchedule(
		uuid.NewString(),
		roadmap.ID(),
		roadmap.UserID(),
		1,
		now,
		"beginner",
		now,
	)
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	event := &domain.Event{
		ID:           uuid.NewString(),
		Type:         domain.EventTypeRoadmapInitiated,
		PartitionKey: roadmap.UserID(),
		Payload:      []byte(`{"roadmapId":"roadmap-1"}`),
		CreatedAt:    now,
	}

	return roadmap, schedule, event
}

func assertRowCount(
	t *testing.T,
	db *gorm.DB,
	table, column, value string,
	want int64,
) {
	t.Helper()

	var got int64
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + column + " = ?"
	if err := db.Raw(query, value).Scan(&got).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Errorf("%s count got = %d, want = %d", table, got, want)
	}
}

func assertCoachingMigrationsApplied(t *testing.T, db *gorm.DB) {
	t.Helper()

	var indexCount int64
	if err := db.Raw(
		`SELECT COUNT(*) FROM pg_indexes
		 WHERE schemaname = 'coaching'
		   AND indexname IN (
			'uq_coaching_active_roadmap_per_user',
			'uq_coaching_outbox_log_event_id'
		   )`,
	).Scan(&indexCount).Error; err != nil {
		t.Fatalf("verify coaching migrations: %v", err)
	}
	if indexCount != 2 {
		t.Fatalf("coaching migrations are not applied; expected hardening indexes")
	}
}

func deleteFixtureRows(
	t *testing.T,
	db *gorm.DB,
	roadmapID, scheduleID, eventID, inboxEventID string,
) {
	t.Helper()

	deletions := []struct {
		query string
		value string
	}{
		{`DELETE FROM coaching.outbox_log WHERE event_id = ?`, inboxEventID},
		{`DELETE FROM coaching.outbox WHERE event_id = ?`, eventID},
		{`DELETE FROM coaching.weekly_schedules WHERE id = ?`, scheduleID},
		{`DELETE FROM coaching.workout_roadmaps WHERE id = ?`, roadmapID},
	}
	for _, deletion := range deletions {
		if deletion.value == "" {
			continue
		}
		if err := db.Exec(deletion.query, deletion.value).Error; err != nil {
			t.Errorf("clean integration fixture: %v", err)
		}
	}
}
