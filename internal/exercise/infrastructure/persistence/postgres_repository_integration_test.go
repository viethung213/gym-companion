//go:build integration

package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// exerciseMigration is the real schema, so CHECK constraints are tested as shipped.
const exerciseMigration = "../../../shared/database/migrations/core/02-exercise.sql"

func integrationExercise(t *testing.T, id, slug string, primary domain.MuscleGroup, secondary ...domain.MuscleGroup) *domain.Exercise {
	t.Helper()

	exercise, err := domain.NewExercise(domain.Info{
		ID:               id,
		Slug:             slug,
		Name:             strings.ReplaceAll(slug, "_", " "),
		PrimaryMuscle:    primary,
		SecondaryMuscles: secondary,
		Equipment:        domain.EquipmentBarbell,
		Category:         domain.CategoryStrength,
		Level:            domain.LevelIntermediate,
		Force:            domain.ForcePush,
		Instructions:     []string{"Step one.", "Step two."},
		Images:           []string{slug + "/0.jpg"},
		Source:           domain.SourceFreeExerciseDB,
		Status:           domain.StatusActive,
	}, time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new exercise: %v", err)
	}
	return exercise
}

func TestPostgresRepository_SaveFindAndOutbox(t *testing.T) {
	db := getExerciseTestDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	exercise := integrationExercise(t, uuid.NewString(), "Barbell_Squat",
		domain.MuscleGroupQuadriceps, domain.MuscleGroupGlutes, domain.MuscleGroupCalves)
	event := &domain.Event{
		ID:           uuid.NewString(),
		Type:         domain.EventTypeExerciseCreated,
		PartitionKey: exercise.Info().ID,
		Payload:      []byte(`{"id":"x"}`),
		CreatedAt:    time.Now().UTC(),
	}

	if err := repo.Save(ctx, exercise, event); err != nil {
		t.Fatalf("save exercise: %v", err)
	}

	loaded, err := repo.FindByID(ctx, exercise.Info().ID)
	if err != nil {
		t.Fatalf("find exercise: %v", err)
	}
	got := loaded.Info()
	if len(got.SecondaryMuscles) != 2 || got.SecondaryMuscles[1] != domain.MuscleGroupCalves {
		t.Errorf("got secondary muscles %v", got.SecondaryMuscles)
	}
	if len(got.Instructions) != 2 || got.Images[0] != "Barbell_Squat/0.jpg" {
		t.Errorf("got instructions %v images %v", got.Instructions, got.Images)
	}
	if got.Force != domain.ForcePush || got.Mechanic != "" {
		t.Errorf("got force %q mechanic %q", got.Force, got.Mechanic)
	}

	var outboxCount int
	if err := db.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM exercise.outbox WHERE event_id = ?`, event.ID).
		Scan(&outboxCount).Error; err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("got outbox count %d, want 1", outboxCount)
	}

	duplicate := integrationExercise(t, uuid.NewString(), "Barbell_Squat", domain.MuscleGroupQuadriceps)
	if err := repo.Save(ctx, duplicate, nil); !errors.Is(err, domain.ErrDuplicateSlug) {
		t.Errorf("duplicate slug: got %v, want %v", err, domain.ErrDuplicateSlug)
	}
}

// TestPostgresRepository_SearchExcludeMuscles pins docs/05 §8.2 item 4 (fixes F1):
// an excluded muscle drops exercises loading it as primary or secondary.
func TestPostgresRepository_SearchExcludeMuscles(t *testing.T) {
	db := getExerciseTestDB(t)
	ctx := context.Background()
	repo := NewPostgresRepository(db)

	squat := integrationExercise(t, uuid.NewString(), "Barbell_Squat", domain.MuscleGroupQuadriceps, domain.MuscleGroupGlutes)
	deadlift := integrationExercise(t, uuid.NewString(), "Barbell_Deadlift", domain.MuscleGroupLowerBack, domain.MuscleGroupHamstrings)
	bench := integrationExercise(t, uuid.NewString(), "Barbell_Bench_Press", domain.MuscleGroupChest, domain.MuscleGroupTriceps)
	for _, ex := range []*domain.Exercise{squat, deadlift, bench} {
		if err := repo.Save(ctx, ex, nil); err != nil {
			t.Fatalf("save %s: %v", ex.Info().Slug, err)
		}
	}

	tests := []struct {
		name    string
		filters port.SearchFilters
		want    []string
	}{
		{
			name:    "exclude primary and secondary",
			filters: port.SearchFilters{ExcludeMuscles: []domain.MuscleGroup{domain.MuscleGroupQuadriceps, domain.MuscleGroupHamstrings}},
			want:    []string{"Barbell_Bench_Press"},
		},
		{
			name:    "primary any of",
			filters: port.SearchFilters{PrimaryMuscles: []domain.MuscleGroup{domain.MuscleGroupChest, domain.MuscleGroupLowerBack}},
			want:    []string{"Barbell_Bench_Press", "Barbell_Deadlift"},
		},
		{
			name:    "secondary overlap",
			filters: port.SearchFilters{SecondaryMuscles: []domain.MuscleGroup{domain.MuscleGroupGlutes}},
			want:    []string{"Barbell_Squat"},
		},
		{
			name: "equipment, category, level and force",
			filters: port.SearchFilters{
				Equipment:  []domain.Equipment{domain.EquipmentBarbell},
				Categories: []domain.Category{domain.CategoryStrength},
				Levels:     []domain.Level{domain.LevelIntermediate},
				Force:      domain.ForcePush,
			},
			want: []string{"Barbell_Bench_Press", "Barbell_Deadlift", "Barbell_Squat"},
		},
		{
			name:    "mechanic filter",
			filters: port.SearchFilters{Mechanic: domain.MechanicCompound},
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exercises, total, err := repo.SearchActive(ctx, &tt.filters)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			var got []string
			for _, ex := range exercises {
				got = append(got, ex.Info().Slug)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) || int(total) != len(tt.want) {
				t.Errorf("got %v (total %d), want %v", got, total, tt.want)
			}
		})
	}
}

// TestExerciseSchema_ChecksRejectInvalidValues verifies the migration CHECKs
// are the last line of defence (docs/05 §7).
func TestExerciseSchema_ChecksRejectInvalidValues(t *testing.T) {
	db := getExerciseTestDB(t)

	tests := []struct {
		name       string
		column     string
		value      string
		constraint string
	}{
		{"primary muscle", "primary_muscle", "'QUADS'", "chk_exercises_primary_muscle"},
		{"secondary muscles", "secondary_muscles", "ARRAY['GLUTES','UPPER_LEGS']", "chk_exercises_secondary_muscles"},
		{"equipment", "equipment", "'body only'", "chk_exercises_equipment"},
		{"category", "category", "'YOGA'", "chk_exercises_category"},
		{"level", "level", "'ADVANCED'", "chk_exercises_level"},
		{"force", "force", "'TWIST'", "chk_exercises_force"},
		{"mechanic", "mechanic", "'HYBRID'", "chk_exercises_mechanic"},
		{"source", "source", "'wger'", "chk_exercises_source"},
		{"ai without motion slug", "has_ai_supported", "TRUE", "chk_exercises_ai_requires_motion_slug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[string]string{
				"primary_muscle":    "'CHEST'",
				"secondary_muscles": "ARRAY['TRICEPS']",
				"equipment":         "'BARBELL'",
				"category":          "'STRENGTH'",
				"level":             "'BEGINNER'",
				"force":             "NULL",
				"mechanic":          "NULL",
				"source":            "'custom'",
				"has_ai_supported":  "FALSE",
			}
			values[tt.column] = tt.value

			stmt := fmt.Sprintf(`INSERT INTO exercise.exercises
				(id, slug, name, primary_muscle, secondary_muscles, equipment, category, level,
				 force, mechanic, source, has_ai_supported)
				VALUES ('%s', '%s', 'x', %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
				uuid.NewString(), uuid.NewString(), values["primary_muscle"], values["secondary_muscles"],
				values["equipment"], values["category"], values["level"], values["force"],
				values["mechanic"], values["source"], values["has_ai_supported"])

			err := db.Exec(stmt).Error
			if err == nil || !strings.Contains(err.Error(), tt.constraint) {
				t.Errorf("got error %v, want violation of %s", err, tt.constraint)
			}
		})
	}
}

func TestPostgresRepository_FindByIDNotFound(t *testing.T) {
	repo := NewPostgresRepository(getExerciseTestDB(t))

	_, err := repo.FindByID(context.Background(), "non-existent-uuid")
	if !errors.Is(err, domain.ErrExerciseNotFound) {
		t.Fatalf("got error %v, want %v", err, domain.ErrExerciseNotFound)
	}
}

// prepareExerciseSchema applies the real migration (idempotent) and empties
// the exercise tables this package writes.
func prepareExerciseSchema(ctx context.Context, db *gorm.DB) error {
	migration, err := os.ReadFile(exerciseMigration)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if err := db.WithContext(ctx).Exec(string(migration)).Error; err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}

	return db.WithContext(ctx).
		Exec(`TRUNCATE TABLE exercise.exercise_stats, exercise.exercises, exercise.outbox CASCADE`).
		Error
}

func InitDB(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	// Set connection pool configurations
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	return db, nil
}

func getExerciseTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	var db *gorm.DB
	if databaseURL := os.Getenv("TEST_DATABASE_URL"); databaseURL != "" {
		var err error
		if db, err = InitDB(databaseURL); err != nil {
			t.Fatalf("open database: %v", err)
		}
	} else {
		sqlDB, err := database.GetRegistry().GetPool("exercise")
		if err != nil {
			t.Fatalf("Failed to initialize exercise test database: %v", err)
		}
		if db, err = gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
			SkipDefaultTransaction: true,
		}); err != nil {
			t.Fatalf("Failed to wrap database in gorm: %v", err)
		}
	}

	if err := prepareExerciseSchema(context.Background(), db); err != nil {
		t.Fatalf("prepare schema: %v", err)
	}
	return db
}

func truncateExerciseOutbox(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE exercise.outbox CASCADE;")
}

func TestExerciseOutboxRepository_3ConcurrentNodes_Integration(t *testing.T) {
	db := getExerciseTestDB(t)
	truncateExerciseOutbox(db)
	defer truncateExerciseOutbox(db)

	repo := NewOutboxRepository(db)
	ctx := context.Background()

	const totalRecords = 30
	const batchLimit = 10

	for i := 0; i < totalRecords; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:           id,
			EventID:      id,
			EventType:    "ExerciseCreated",
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
				_ = repo.MarkPublished(ctx, recIDs)
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

func TestExerciseOutboxRepository_NodeCrashFailover_Integration(t *testing.T) {
	db := getExerciseTestDB(t)
	truncateExerciseOutbox(db)
	defer truncateExerciseOutbox(db)

	repo := NewOutboxRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:           id,
			EventID:      id,
			EventType:    "ExerciseCreated",
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

	// Node 1 CRASHES (no MarkPublished)

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
	if err := repo.MarkPublished(ctx, recIDs); err != nil {
		t.Fatalf("Node 2 MarkPublished failed: %v", err)
	}

	unpub, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished failed: %v", err)
	}
	if len(unpub) != 0 {
		t.Errorf("Expected 0 leftover unpublished records, got %d", len(unpub))
	}
}
