package seeder

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

// Every listed exercise exists upstream, so a typo cannot silently skip one,
// and the listed muscle lands among its secondary muscles once.
func TestStabilizers(t *testing.T) {
	extra, err := loadStabilizers()
	if err != nil {
		t.Fatal(err)
	}
	var records []upstreamExercise
	if err := json.Unmarshal(upstreamExercisesJSON, &records); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]*upstreamExercise, len(records))
	for i := range records {
		byID[records[i].ID] = &records[i]
	}
	for id := range extra {
		if byID[id] == nil {
			t.Errorf("%s is not an upstream exercise", id)
		}
	}

	row := *byID["Bent_Over_Two-Dumbbell_Row"]
	withStabilizers(&row, extra)
	withStabilizers(&row, extra)
	if n := countOf(row.SecondaryMuscles, "lower back"); n != 1 {
		t.Errorf("secondary %v, want lower back once", row.SecondaryMuscles)
	}
	req, err := newCreateExerciseRequest(&row, "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(req.GetSecondaryMuscles(), exercisemsg.MuscleGroup_MUSCLE_GROUP_LOWER_BACK) {
		t.Errorf("request secondary %v, want LOWER_BACK", req.GetSecondaryMuscles())
	}
}

// Exercises seeded before the list existed are patched by the core
// migration, so it must name every listed exercise.
func TestStabilizersBackfilledByMigration(t *testing.T) {
	extra, err := loadStabilizers()
	if err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile("../database/migrations/core/02-exercise.sql")
	if err != nil {
		t.Fatal(err)
	}
	for id := range extra {
		if !strings.Contains(string(sql), "'"+id+"'") {
			t.Errorf("02-exercise.sql does not backfill %s", id)
		}
	}
}

func countOf(xs []string, x string) int {
	n := 0
	for _, v := range xs {
		if v == x {
			n++
		}
	}
	return n
}
