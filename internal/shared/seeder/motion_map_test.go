package seeder

import (
	"encoding/json"
	"testing"
)

// TestAIMotionMap_MatchesCameraAngles keeps data/ai_motion_map.json and the
// per-slug camera angles in step: 12 AI motions, each with a camera angle.
func TestAIMotionMap_MatchesCameraAngles(t *testing.T) {
	var bySlug map[string][]string
	if err := json.Unmarshal(aiMotionMapJSON, &bySlug); err != nil {
		t.Fatalf("unmarshal ai motion map: %v", err)
	}

	angles := motionCameraAngles()
	if len(bySlug) != 12 || len(angles) != 12 {
		t.Fatalf("got %d mapped slugs and %d camera angles, want 12 each", len(bySlug), len(angles))
	}
	for slug, exercises := range bySlug {
		if _, ok := angles[slug]; !ok {
			t.Errorf("motion slug %q has no camera angle", slug)
		}
		if len(exercises) == 0 {
			t.Errorf("motion slug %q links no exercise", slug)
		}
	}
}
