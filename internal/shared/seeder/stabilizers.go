package seeder

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
)

// stabilizerMusclesJSON maps an upstream muscle to the source exercise ids
// that load it without free-exercise-db listing it. The source lists the
// muscles that move; a standing hip hinge (bent-over row, deadlift, swing)
// also loads the lower back holding the torso, and injury screening must see
// that load. Rows seeded before an id was listed are patched by the core
// migration 02-exercise.sql.
//
//go:embed data/stabilizer_muscles.json
var stabilizerMusclesJSON []byte

// loadStabilizers returns source exercise id -> extra upstream muscles.
func loadStabilizers() (map[string][]string, error) {
	var byMuscle map[string][]string
	if err := json.Unmarshal(stabilizerMusclesJSON, &byMuscle); err != nil {
		return nil, fmt.Errorf("unmarshal stabilizer muscles: %w", err)
	}
	byExercise := make(map[string][]string)
	for muscle, ids := range byMuscle {
		for _, id := range ids {
			byExercise[id] = append(byExercise[id], muscle)
		}
	}
	return byExercise, nil
}

// withStabilizers adds the exercise's missing stabilizer muscles to its
// secondary muscles.
func withStabilizers(rec *upstreamExercise, extra map[string][]string) {
	for _, m := range extra[rec.ID] {
		if !slices.Contains(rec.PrimaryMuscles, m) && !slices.Contains(rec.SecondaryMuscles, m) {
			rec.SecondaryMuscles = append(rec.SecondaryMuscles, m)
		}
	}
}
