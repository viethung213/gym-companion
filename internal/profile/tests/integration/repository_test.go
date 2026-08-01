//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	infraPostgres "github.com/viethung213/gym-companion/internal/profile/infrastructure/persistence"
	"github.com/viethung213/gym-companion/internal/shared/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func getTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	sqlDB, err := database.GetRegistry().GetPool("profile")
	if err != nil {
		t.Fatalf("Integration test failed to get profile database pool: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("Integration test failed to wrap database in gorm: %v", err)
	}

	ensureTablesExist(db)
	return db
}

func ensureTablesExist(db *gorm.DB) {
	db.Exec(`CREATE SCHEMA IF NOT EXISTS profile;`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.users (
		user_id VARCHAR(255) PRIMARY KEY,
		date_of_birth TIMESTAMP WITH TIME ZONE,
		gender VARCHAR(50),
		experience_level VARCHAR(50),
		goals JSONB,
		preferred_workout_times JSONB,
		available_equipment JSONB,
		preferred_muscle_groups JSONB,
		coach_style VARCHAR(50),
		target_weight_kg NUMERIC(10, 2),
		target_body_fat_percent NUMERIC(5, 2),
		completion_rate NUMERIC(5, 2),
		ai_coach_activated BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.body_metrics (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL REFERENCES profile.users(user_id) ON DELETE CASCADE,
		weight_kg NUMERIC(10, 2),
		height_cm NUMERIC(10, 2),
		body_fat_percent NUMERIC(5, 2),
		progress_photo_url VARCHAR(1024),
		logged_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.injuries (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL REFERENCES profile.users(user_id) ON DELETE CASCADE,
		muscle_group VARCHAR(100) NOT NULL,
		severity VARCHAR(50) NOT NULL,
		notes TEXT,
		reported_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		recovered_at TIMESTAMP WITH TIME ZONE
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.outbox (
		id VARCHAR(255) PRIMARY KEY,
		event_id VARCHAR(255),
		aggregate_type VARCHAR(255),
		aggregate_id VARCHAR(255),
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		partition_key VARCHAR(255),
		published BOOLEAN DEFAULT FALSE,
		published_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		status VARCHAR(50) DEFAULT 'PENDING' NOT NULL,
		locked_until TIMESTAMP WITH TIME ZONE
	);`)
	db.Exec("ALTER TABLE profile.outbox ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'PENDING' NOT NULL;")
	db.Exec("ALTER TABLE profile.outbox ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP WITH TIME ZONE;")

	db.Exec(`CREATE TABLE IF NOT EXISTS profile.outbox_log (
		id UUID PRIMARY KEY,
		event_id UUID NOT NULL,
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		partition_key VARCHAR(255) NOT NULL,
		processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		status VARCHAR(50) NOT NULL,
		error_message TEXT
	);`)
}

func truncateTables(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE profile.injuries CASCADE")
	db.Exec("TRUNCATE TABLE profile.body_metrics CASCADE")
	db.Exec("TRUNCATE TABLE profile.users CASCADE")
	db.Exec("TRUNCATE TABLE profile.outbox CASCADE")
	db.Exec("TRUNCATE TABLE profile.outbox_log CASCADE")
}

func TestPostgresUserProfileRepository_SaveAndFindByUserID(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewPostgresUserProfileRepository(db)
	ctx := context.Background()

	userID := uuid.NewString()
	dob := time.Date(1996, 8, 15, 0, 0, 0, 0, time.UTC)
	bioMetrics, err := vo.NewBiologicalMetricsWithDOB(75.5, 178.0, dob, "MALE")
	if err != nil {
		t.Fatalf("NewBiologicalMetricsWithDOB failed: %v", err)
	}

	userProfile, err := aggregate.NewUserProfile(
		userID,
		bioMetrics,
		"INTERMEDIATE",
		[]string{"BUILD_MUSCLE"},
		[]string{"MORNING"},
		[]string{"BARBELL"},
		[]string{"CHEST"},
		"FRIENDLY",
		80.0,
		15.0,
		nil,
	)
	if err != nil {
		t.Fatalf("NewUserProfile failed: %v", err)
	}

	// Update Preferences & Target Metrics
	userProfile.UpdateProfile(
		bioMetrics,
		"ADVANCED",
		[]string{"BUILD_MUSCLE"},
		[]string{"MORNING"},
		[]string{"BARBELL", "DUMBBELL"},
		[]string{"CHEST", "ARMS"},
		"MOTIVATIONAL",
		82.0,
		14.0,
	)

	// Add Injury
	inj, err := entity.NewInjury(uuid.NewString(), "SHOULDER", "MODERATE", "Overuse strain", time.Now().UTC())
	if err != nil {
		t.Fatalf("NewInjury failed: %v", err)
	}
	if err := userProfile.AddInjury(inj); err != nil {
		t.Fatalf("AddInjury failed: %v", err)
	}

	// Save profile to database
	if err := repo.Save(ctx, userProfile); err != nil {
		t.Fatalf("Save profile failed: %v", err)
	}

	// Find profile from database
	fetched, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID failed: %v", err)
	}

	if fetched.UserID() != userID {
		t.Errorf("got UserID %s, want %s", fetched.UserID(), userID)
	}
	if fetched.ExperienceLevel() != "ADVANCED" {
		t.Errorf("got ExperienceLevel %s, want ADVANCED", fetched.ExperienceLevel())
	}
	if fetched.CoachStyle() != "MOTIVATIONAL" {
		t.Errorf("got CoachStyle %s, want MOTIVATIONAL", fetched.CoachStyle())
	}
	if len(fetched.Injuries()) != 1 {
		t.Errorf("got %d injuries, want 1", len(fetched.Injuries()))
	}
	if fetched.Injuries()[0].MuscleGroup() != "SHOULDER" {
		t.Errorf("got injury muscle group %s, want SHOULDER", fetched.Injuries()[0].MuscleGroup())
	}
}

func TestPostgresUserProfileRepository_TransactionManager(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewPostgresUserProfileRepository(db)
	outboxRepo := infraPostgres.NewGormOutboxRepository(db)
	txManager := infraPostgres.NewSQLTransactionManager(db)

	userID := uuid.NewString()
	dob := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	bioMetrics, _ := vo.NewBiologicalMetricsWithDOB(60.0, 165.0, dob, "FEMALE")
	userProfile, _ := aggregate.NewUserProfile(
		userID,
		bioMetrics,
		"BEGINNER",
		[]string{"LOSE_WEIGHT"},
		[]string{"EVENING"},
		[]string{"DUMBBELL"},
		[]string{"LEGS"},
		"FRIENDLY",
		55.0,
		20.0,
		nil,
	)

	ctx := context.Background()

	// Perform atomic transaction writing both profile and outbox event
	err := txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.Save(txCtx, userProfile); err != nil {
			return err
		}
		outboxRecord := &port.OutboxRecord{
			ID:           uuid.NewString(),
			EventID:      uuid.NewString(),
			EventType:    "contracts.supporting.profile.v1.userProfileCreated",
			Payload:      []byte(`{"userId":"` + userID + `"}`),
			PartitionKey: userID,
		}
		return outboxRepo.Save(txCtx, outboxRecord)
	})
	if err != nil {
		t.Fatalf("WithTransaction failed: %v", err)
	}

	// Verify profile exists in DB
	fetched, err := repo.FindByUserID(ctx, userID)
	if err != nil || fetched == nil {
		t.Fatalf("Failed to fetch profile committed in transaction: %v", err)
	}

	// Verify outbox record exists in DB
	records, err := outboxRepo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d unpublished outbox records, want 1", len(records))
	}
}

func TestProfileOutboxRepository_3ConcurrentNodes_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewGormOutboxRepository(db)
	ctx := context.Background()

	const totalRecords = 30
	const batchLimit = 10

	for i := 0; i < totalRecords; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:           id,
			EventID:      id,
			EventType:    "ProfileCompleted",
			Payload:      []byte(`{}`),
			PartitionKey: fmt.Sprintf("key-%d", i),
		}
		err := repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("Failed to seed record %d: %v", i, err)
		}
	}

	var mu sync.Mutex
	processedIDs := make(map[string]int)

	var wg sync.WaitGroup
	const nodeCount = 3
	wg.Add(nodeCount)

	for nodeIdx := 1; nodeIdx <= nodeCount; nodeIdx++ {
		go func(id int) {
			defer wg.Done()
			for {
				records, err := repo.ClaimBatch(ctx, batchLimit, 5*time.Second)
				if err != nil || len(records) == 0 {
					break
				}

				mu.Lock()
				recIDs := make([]string, len(records))
				for i, r := range records {
					processedIDs[r.ID]++
					recIDs[i] = r.ID
				}
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
				_ = repo.MarkAsPublished(ctx, recIDs)
			}
		}(nodeIdx)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(processedIDs) != totalRecords {
		t.Errorf("Expected %d total unique records processed across 3 nodes, got %d", totalRecords, len(processedIDs))
	}

	for id, count := range processedIDs {
		if count > 1 {
			t.Errorf("Record %s was processed %d times (expected exactly 1)", id, count)
		}
	}
}

func TestProfileOutboxRepository_NodeCrashFailover_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewGormOutboxRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:           id,
			EventID:      id,
			EventType:    "ProfileCompleted",
			Payload:      []byte(`{}`),
			PartitionKey: fmt.Sprintf("key-%d", i),
		}
		err := repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("Failed to seed record %d: %v", i, err)
		}
	}

	// Node 1 claims with 100ms lock
	node1Claimed, err := repo.ClaimBatch(ctx, 5, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Node 1 ClaimBatch failed: %v", err)
	}
	if len(node1Claimed) != 5 {
		t.Fatalf("Node 1 expected 5 claimed records, got %d", len(node1Claimed))
	}

	// Node 1 CRASHES (no MarkAsPublished)

	// Node 2 tries immediately -> 0 records
	node2Immediate, err := repo.ClaimBatch(ctx, 5, 5*time.Second)
	if err != nil {
		t.Fatalf("Node 2 immediate ClaimBatch failed: %v", err)
	}
	if len(node2Immediate) != 0 {
		t.Errorf("Node 2 expected 0 records while Node 1 lock active, got %d", len(node2Immediate))
	}

	// Wait 150ms for lock expiration
	time.Sleep(150 * time.Millisecond)

	// Node 2 tries after lock expired -> claims all 5
	node2Recovered, err := repo.ClaimBatch(ctx, 5, 5*time.Second)
	if err != nil {
		t.Fatalf("Node 2 failover ClaimBatch failed: %v", err)
	}
	if len(node2Recovered) != 5 {
		t.Errorf("Node 2 expected 5 recovered records after Node 1 crash, got %d", len(node2Recovered))
	}

	recIDs := make([]string, len(node2Recovered))
	for i, r := range node2Recovered {
		recIDs[i] = r.ID
	}
	if err := repo.MarkAsPublished(ctx, recIDs); err != nil {
		t.Fatalf("Node 2 MarkAsPublished failed: %v", err)
	}

	unpub, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished failed: %v", err)
	}
	if len(unpub) != 0 {
		t.Errorf("Expected 0 leftover unpublished records, got %d", len(unpub))
	}
}
