//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/shared/database"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
)

// TestSeedExercises_RealCatalog_E2E seeds the vendored free-exercise-db catalog
// through the real Exercise service twice (docs/05 §9).
func TestSeedExercises_RealCatalog_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	sqlDB, err := database.GetRegistry().GetPool("exercise")
	if err != nil {
		t.Fatalf("exercise pool: %v", err)
	}
	ctx := context.Background()

	for run := 1; run <= 2; run++ {
		if err := seeder.SeedExercises(ctx, sqlDB, suite.Server); err != nil {
			t.Fatalf("seed run %d: %v", run, err)
		}
	}

	var stats struct {
		Total      int
		Distinct   int
		Drafts     int
		Linked     int
		MotionSlug int
		BodyOnly   int
	}
	err = suite.DB.Raw(`SELECT
			COUNT(*) AS total,
			COUNT(DISTINCT slug) AS distinct,
			COUNT(*) FILTER (WHERE status = 'DRAFT') AS drafts,
			COUNT(*) FILTER (WHERE motion_slug IS NOT NULL) AS linked,
			COUNT(DISTINCT motion_slug) AS motion_slug,
			COUNT(*) FILTER (WHERE equipment = 'BODY_ONLY') AS body_only
		FROM exercise.exercises`).Scan(&stats).Error
	if err != nil {
		t.Fatalf("query stats: %v", err)
	}

	want := struct {
		Total      int
		Distinct   int
		Drafts     int
		Linked     int
		MotionSlug int
		BodyOnly   int
	}{Total: 876, Distinct: 876, Drafts: 3, Linked: 16, MotionSlug: 12, BodyOnly: 188}
	if stats != want {
		t.Errorf("got %+v, want %+v", stats, want)
	}

	var outbox int
	if err := suite.DB.Raw(`SELECT COUNT(*) FROM exercise.outbox WHERE event_type = 'contracts.supporting.exercise.v1.exerciseCreated'`).
		Scan(&outbox).Error; err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outbox != 876 {
		t.Errorf("got %d ExerciseCreated events, want 876 (one per created exercise)", outbox)
	}
}
