package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

// MotionAssetType constants
const (
	AssetTypePoseRules      = "POSE_RULES"
	AssetTypeDialogueConfig = "DIALOGUE_CONFIG"
)

// PatchMotionSpecificationAssetCommand holds parameters to perform partial patch on motion asset JSON on S3.
type PatchMotionSpecificationAssetCommand struct {
	ExerciseID string
	AssetType  string
	PatchJSON  string
	DeleteKeys []string
}

// PatchMotionSpecificationAssetResult contains response metadata after partial patching.
type PatchMotionSpecificationAssetResult struct {
	ExerciseID string
	AssetType  string
	FileURL    string
	Spec       *aggregate.MotionSpecification
}

// PatchMotionSpecificationAssetHandler handles partial JSON patching on S3 asset files.
type PatchMotionSpecificationAssetHandler struct {
	motionRepo      repository.MotionSpecificationRepository
	storageProvider port.ObjectStorageProvider
	outbox          port.OutboxWriter
	txManager       port.TxManager
}

// NewPatchMotionSpecificationAssetHandler constructs the handler.
func NewPatchMotionSpecificationAssetHandler(
	motionRepo repository.MotionSpecificationRepository,
	storageProvider port.ObjectStorageProvider,
	outbox port.OutboxWriter,
	txManager port.TxManager,
) *PatchMotionSpecificationAssetHandler {
	return &PatchMotionSpecificationAssetHandler{
		motionRepo:      motionRepo,
		storageProvider: storageProvider,
		outbox:          outbox,
		txManager:       txManager,
	}
}

// Handle executes the Partial JSON Patching workflow.
func (h *PatchMotionSpecificationAssetHandler) Handle(
	ctx context.Context,
	cmd PatchMotionSpecificationAssetCommand,
) (*PatchMotionSpecificationAssetResult, error) {
	exerciseID := strings.TrimSpace(cmd.ExerciseID)
	if exerciseID == "" {
		return nil, fmt.Errorf("patch motion asset: %w: exercise_id is required", apperror.ErrInvalidInput)
	}

	assetTypeNormalized := strings.ToUpper(strings.TrimSpace(cmd.AssetType))
	if assetTypeNormalized == "1" || assetTypeNormalized == "MOTION_ASSET_TYPE_POSE_RULES" || assetTypeNormalized == "POSE_RULES" {
		assetTypeNormalized = AssetTypePoseRules
	} else if assetTypeNormalized == "2" || assetTypeNormalized == "MOTION_ASSET_TYPE_DIALOGUE_CONFIG" || assetTypeNormalized == "DIALOGUE_CONFIG" {
		assetTypeNormalized = AssetTypeDialogueConfig
	}

	if assetTypeNormalized != AssetTypePoseRules && assetTypeNormalized != AssetTypeDialogueConfig {
		return nil, fmt.Errorf("patch motion asset: %w: invalid asset_type (must be MOTION_ASSET_TYPE_POSE_RULES or MOTION_ASSET_TYPE_DIALOGUE_CONFIG)", apperror.ErrInvalidInput)
	}

	// 1. Check if MotionSpecification exists in Database
	spec, err := h.motionRepo.FindByExerciseID(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, derror.ErrNotFound) || errors.Is(err, derror.ErrMotionSpecNotFound) {
			return nil, fmt.Errorf("patch motion asset: %w: exercise_id '%s'", derror.ErrMotionSpecNotFound, exerciseID)
		}
		return nil, fmt.Errorf("patch motion asset find spec: %w", err)
	}

	// 2. Determine existing URL and S3 Object Key
	var existingURL string
	if assetTypeNormalized == AssetTypePoseRules {
		existingURL = spec.LocalRulesURL()
	} else {
		existingURL = spec.DialogueEngineURL()
	}
	objectKey := deriveObjectKeyFromURL(existingURL, assetTypeNormalized, exerciseID)

	// 3. Fetch existing JSON from S3 or via HTTP GET fallback
	existingMap := make(map[string]interface{})
	var rawBytes []byte
	if h.storageProvider != nil {
		rawBytes, _ = h.storageProvider.GetObject(ctx, objectKey)
	}

	// Fallback to HTTP GET if S3 GetObject returned empty but an existing HTTP URL exists
	if len(rawBytes) == 0 && strings.HasPrefix(existingURL, "http") {
		req, httpErr := http.NewRequestWithContext(ctx, http.MethodGet, existingURL, nil)
		if httpErr == nil {
			resp, doErr := http.DefaultClient.Do(req)
			if doErr == nil && resp.StatusCode == http.StatusOK {
				data, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr == nil && len(data) > 0 {
					rawBytes = data
				}
			}
		}
	}

	if len(rawBytes) > 0 {
		_ = json.Unmarshal(rawBytes, &existingMap)
	}

	// 4. Apply Patch JSON if provided
	if strings.TrimSpace(cmd.PatchJSON) != "" {
		var patchMap map[string]interface{}
		if err := json.Unmarshal([]byte(cmd.PatchJSON), &patchMap); err != nil {
			return nil, fmt.Errorf("patch motion asset: %w: invalid patch_json format: %w", apperror.ErrInvalidInput, err)
		}
		deepMergeMaps(existingMap, patchMap)
	}

	// 5. Delete specified keys if provided
	for _, k := range cmd.DeleteKeys {
		deleteNestedKey(existingMap, k)
	}

	// 6. Marshal updated map back to JSON
	updatedBytes, err := json.MarshalIndent(existingMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("patch motion asset marshal: %w", err)
	}

	// 7. Put updated JSON file back to S3
	var publicFileURL string
	if h.storageProvider != nil {
		publicFileURL, err = h.storageProvider.PutObject(ctx, objectKey, updatedBytes, "application/json")
		if err != nil {
			return nil, fmt.Errorf("patch motion asset upload: %w", err)
		}
	} else {
		publicFileURL = fmt.Sprintf("https://cdn.fitai.com/%s", objectKey)
	}

	rulesURL := spec.LocalRulesURL()
	dialogueURL := spec.DialogueEngineURL()

	if assetTypeNormalized == AssetTypePoseRules {
		rulesURL = publicFileURL
	} else {
		dialogueURL = publicFileURL
	}

	spec.UpdateSpec(spec.OnnxDetectorURL(), spec.OnnxSkeletonURL(), rulesURL, dialogueURL, spec.RecommendedCameraAngle())

	// 8. Persist spec & domain events
	var saveErr error
	if h.txManager != nil {
		saveErr = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := h.motionRepo.Save(txCtx, spec); err != nil {
				return err
			}
			events := spec.PopEvents()
			if len(events) > 0 && h.outbox != nil {
				if err := h.outbox.WriteEvents(txCtx, "MotionSpecification", spec.ExerciseID(), events); err != nil {
					return err
				}
			}
			return nil
		})
	} else {
		saveErr = h.motionRepo.Save(ctx, spec)
	}

	if saveErr != nil {
		return nil, fmt.Errorf("patch motion asset save spec: %w", saveErr)
	}

	return &PatchMotionSpecificationAssetResult{
		ExerciseID: exerciseID,
		AssetType:  assetTypeNormalized,
		FileURL:    publicFileURL,
		Spec:       spec,
	}, nil
}

// deepMergeMaps recursively merges src into dst.
func deepMergeMaps(dst, src map[string]interface{}) {
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				deepMergeMaps(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
}

// deleteNestedKey deletes key by dot-notation path (e.g. "cooldowns.ERR_KNEE_OVER_TOE").
func deleteNestedKey(m map[string]interface{}, path string) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return
	}

	curr := m
	for i := 0; i < len(parts)-1; i++ {
		next, ok := curr[parts[i]].(map[string]interface{})
		if !ok {
			return
		}
		curr = next
	}

	delete(curr, parts[len(parts)-1])
}

// deriveObjectKeyFromURL extracts the S3 object key path from an existing URL or generates a standard default.
func deriveObjectKeyFromURL(existingURL, assetType, exerciseID string) string {
	if existingURL != "" {
		clean := strings.TrimSpace(existingURL)
		if idx := strings.Index(clean, "/gym-companion-assets/"); idx != -1 {
			return strings.TrimPrefix(clean[idx+len("/gym-companion-assets/"):], "/")
		}
		if idx := strings.Index(clean, "/public/"); idx != -1 {
			sub := clean[idx+len("/public/"):]
			parts := strings.SplitN(sub, "/", 2)
			if len(parts) == 2 {
				return strings.TrimPrefix(parts[1], "/")
			}
		}
		if !strings.HasPrefix(clean, "http://") && !strings.HasPrefix(clean, "https://") {
			return strings.TrimPrefix(clean, "/")
		}
	}

	if assetType == AssetTypePoseRules {
		return fmt.Sprintf("rules/%s_rule.json", exerciseID)
	}
	return fmt.Sprintf("dialogues/%s_dialogue.json", exerciseID)
}
