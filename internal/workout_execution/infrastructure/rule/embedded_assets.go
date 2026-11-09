package rule

import (
	"embed"
	"fmt"
	"path"
	"strings"
)

//go:embed assets/*/*.json
var embeddedAssetsFS embed.FS

// GetEmbeddedExerciseAsset returns the embedded JSON bytes for a canonical exercise rule or dialogue.
func GetEmbeddedExerciseAsset(assetType, exerciseID string) ([]byte, bool) {
	return loadEmbeddedExerciseAsset(assetType, exerciseID, "")
}

func loadEmbeddedExerciseAsset(assetType, exerciseID, fileURL string) ([]byte, bool) {
	slug := normalizeExerciseSlug(exerciseID, fileURL)
	if slug == "" {
		return nil, false
	}

	var filePath string
	if assetType == "rules" {
		filePath = fmt.Sprintf("assets/rules/%s_rule.json", slug)
	} else {
		filePath = fmt.Sprintf("assets/dialogues/%s_dialogue.json", slug)
	}

	data, err := embeddedAssetsFS.ReadFile(filePath)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

func normalizeExerciseSlug(exerciseID, fileURL string) string {
	if fileURL != "" {
		base := path.Base(strings.TrimSpace(fileURL))
		base = strings.TrimSuffix(base, "_rule.json")
		base = strings.TrimSuffix(base, "_dialogue.json")
		base = strings.TrimSuffix(base, ".json")
		if isKnownExerciseSlug(base) {
			return base
		}
	}

	clean := strings.ToLower(strings.TrimSpace(exerciseID))
	clean = strings.ReplaceAll(clean, "-", "_")
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, "/", "_")
	clean = strings.ReplaceAll(clean, ".", "")
	switch clean {
	case "barbell_bench_press":
		return "bench_press"
	case "barbell_deadlift", "romanian_deadlift":
		return "deadlift"
	case "pushup", "push_up":
		return "push_up"
	case "pullup", "pull_up":
		return "pull_up"
	case "situp", "sit_up", "3_4_sit_up":
		return "sit_up"
	case "dumbbell_biceps_curl", "dumbbell_bicep_curl", "biceps_curl", "bicep_curl":
		return "bicep_curl"
	case "dumbbell_lateral_raise", "lateral_raise":
		return "lateral_raise"
	case "barbell_lunge", "dumbbell_lunge", "lunge":
		return "lunge"
	case "weighted_squat", "barbell_full_squat", "squat":
		return "squat"
	case "weighted_front_plank", "front_plank_with_twist", "front_plank", "plank":
		return "plank"
	case "dumbbell_seated_shoulder_press", "cable_shoulder_press", "shoulder_press":
		return "shoulder_press"
	case "triceps_dip", "tricep_dip", "triceps_dips", "tricep_dips":
		return "triceps_dip"
	}
	if isKnownExerciseSlug(clean) {
		return clean
	}
	return ""
}

func isKnownExerciseSlug(slug string) bool {
	switch slug {
	case "squat",
		"lunge",
		"deadlift",
		"push_up",
		"bench_press",
		"shoulder_press",
		"pull_up",
		"bicep_curl",
		"lateral_raise",
		"triceps_dip",
		"sit_up",
		"plank":
		return true
	default:
		return false
	}
}
