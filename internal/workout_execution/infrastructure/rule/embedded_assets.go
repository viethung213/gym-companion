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
	case "barbell_bench_press", "24e6981b_60df_42e2_9567_74559290f6b1":
		return "bench_press"
	case "barbell_deadlift", "romanian_deadlift", "796e2f8c_c015_4722_a403_7f00c00ff267":
		return "deadlift"
	case "pushup", "push_up", "e1450892_e272_4680_aa1d_9c5971534026":
		return "push_up"
	case "pullup", "pull_up", "3564c8b1_21f9_4498_896d_139319808cf7":
		return "pull_up"
	case "situp", "sit_up", "3_4_sit_up", "fe1ef684_3071_4522_b0fb_338aeeb26879":
		return "sit_up"
	case "dumbbell_biceps_curl", "dumbbell_bicep_curl", "biceps_curl", "bicep_curl", "e99ba23f_3450_4897_bf20_42618dded05a":
		return "bicep_curl"
	case "dumbbell_lateral_raise", "lateral_raise", "780ae011_6a55_4add_8291_336068c5f1ec":
		return "lateral_raise"
	case "barbell_lunge", "dumbbell_lunge", "lunge", "11c12ccc_e852_42dd_8d4a_afaee2e72314", "2b36300a_cec2_41b3_b078_fa37a52494f0":
		return "lunge"
	case "weighted_squat", "barbell_full_squat", "squat", "24776ced_3998_4529_a7a1_9333fddea59f", "0d8586c7_7dbf_45b5_b52a_7b76e3ee952b":
		return "squat"
	case "weighted_front_plank", "front_plank_with_twist", "front_plank", "plank", "126d95a8_39b3_4ced_8d04_b2d5a2bd2cee", "183541f1_4534_4b9e_a65b_af691519727e":
		return "plank"
	case "dumbbell_seated_shoulder_press", "cable_shoulder_press", "shoulder_press", "c8561d45_ae21_4ffb_ae0e_342699557d9c", "a8721d2e_0d57_4d55_bf04_664987c3557d":
		return "shoulder_press"
	case "triceps_dip", "tricep_dip", "triceps_dips", "tricep_dips", "889cec24_1403_4479_aed9_9319eb1128a7":
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
