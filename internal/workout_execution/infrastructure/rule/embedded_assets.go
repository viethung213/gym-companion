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
	switch clean {
	case "barbell_bench_press":
		return "bench_press"
	case "barbell_deadlift", "romanian_deadlift":
		return "deadlift"
	case "pushup":
		return "push_up"
	case "pullup":
		return "pull_up"
	case "situp":
		return "sit_up"
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
