package command_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
)

type mockStorageProviderForPatch struct {
	storage map[string][]byte
}

func newMockStorageProviderForPatch() *mockStorageProviderForPatch {
	return &mockStorageProviderForPatch{
		storage: make(map[string][]byte),
	}
}

func (m *mockStorageProviderForPatch) GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string) (string, string, error) {
	return "http://upload/" + fileName, "http://cdn/" + fileName, nil
}

func (m *mockStorageProviderForPatch) GetObject(ctx context.Context, objectKey string) ([]byte, error) {
	data, ok := m.storage[objectKey]
	if !ok {
		return []byte("{}"), nil
	}
	return data, nil
}

func (m *mockStorageProviderForPatch) PutObject(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {
	m.storage[objectKey] = data
	return "http://cdn/" + objectKey, nil
}

func TestPatchMotionSpecificationAssetHandler(t *testing.T) {
	repo := newMockMotionSpecRepo()
	storage := newMockStorageProviderForPatch()
	handler := command.NewPatchMotionSpecificationAssetHandler(repo, storage, nil, nil)

	t.Run("Patch dialogue config creates file on S3 and updates spec", func(t *testing.T) {
		draft := aggregate.NewDraftMotionSpecification("ex-squat", "", "")
		_ = repo.Save(context.Background(), draft)

		patchJSON := `{
			"personality_id": "FRIENDLY",
			"cooldowns": {
				"ERR_BACK_ARCH": 5.0
			},
			"dialogue_map": {
				"ERR_BACK_ARCH": {
					"severity_1": [
						{"text": "Keep back straight", "audio_url": "http://cdn/audios/back.mp3"}
					]
				}
			}
		}`

		cmd := command.PatchMotionSpecificationAssetCommand{
			ExerciseID: "ex-squat",
			AssetType:  command.AssetTypeDialogueConfig,
			PatchJSON:  patchJSON,
		}

		res, err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.FileURL != "http://cdn/dialogues/ex-squat_dialogue.json" {
			t.Errorf("got FileURL = %s, want http://cdn/dialogues/ex-squat_dialogue.json", res.FileURL)
		}

		if res.Spec.DialogueEngineURL() != res.FileURL {
			t.Errorf("got DialogueEngineURL = %s, want %s", res.Spec.DialogueEngineURL(), res.FileURL)
		}

		// Verify contents stored in mock storage
		storedData := storage.storage["dialogues/ex-squat_dialogue.json"]

		var parsed map[string]interface{}
		_ = json.Unmarshal(storedData, &parsed)
		if parsed["personality_id"] != "FRIENDLY" {
			t.Errorf("got personality_id = %v, want FRIENDLY", parsed["personality_id"])
		}
	})

	t.Run("Partial patch updates existing keys and deletes specified key", func(t *testing.T) {
		// Seed initial rule JSON in mock storage
		storage.storage["rules/ex-squat_rule.json"] = []byte(`{
			"camera_angle": "side",
			"rules": {
				"knee_angle_min": 80.0,
				"deprecated_rule": true
			}
		}`)
		existingSpec := aggregate.RestoreMotionSpecification("ex-squat", "http://cdn/detector.onnx", "http://cdn/skeleton.onnx", "http://cdn/rules/ex-squat_rule.json", "http://cdn/dialogue/ex-squat_dialogue.json", "side", true, time.Now().UTC(), time.Now().UTC())
		_ = repo.Save(context.Background(), existingSpec)

		patchJSON := `{
			"rules": {
				"knee_angle_min": 75.0,
				"back_angle_max": 45.0
			}
		}`

		cmd := command.PatchMotionSpecificationAssetCommand{
			ExerciseID: "ex-squat",
			AssetType:  command.AssetTypePoseRules,
			PatchJSON:  patchJSON,
			DeleteKeys: []string{"rules.deprecated_rule"},
		}

		res, err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !res.Spec.IsReady() {
			t.Error("want spec.IsReady = true after patching")
		}

		// Verify stored JSON
		storedData := storage.storage["rules/ex-squat_rule.json"]
		var parsed map[string]interface{}
		_ = json.Unmarshal(storedData, &parsed)

		rules, _ := parsed["rules"].(map[string]interface{})
		if rules["knee_angle_min"] != 75.0 {
			t.Errorf("got knee_angle_min = %v, want 75", rules["knee_angle_min"])
		}
		if rules["back_angle_max"] != 45.0 {
			t.Errorf("got back_angle_max = %v, want 45", rules["back_angle_max"])
		}
		if _, exists := rules["deprecated_rule"]; exists {
			t.Error("want deprecated_rule to be deleted from JSON")
		}
	})
}
