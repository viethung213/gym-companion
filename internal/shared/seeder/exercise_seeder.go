package seeder

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

//go:embed data/catalog_metadata_seed.json
var catalogMetadataSeedJSON []byte

//go:embed data/exercises_seed.json
var exercisesSeedJSON []byte

// catalogMetadataSeed defines the structure of master lookup tables.
type catalogMetadataSeed struct {
	BodyParts  []catalogItem       `json:"body_parts"`
	Equipments []catalogItem       `json:"equipments"`
	Muscles    []catalogMuscleItem `json:"muscles"`
	Tags       []catalogItem       `json:"tags"`
}

type catalogItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type catalogMuscleItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BodyPartID string `json:"body_part_id"`
}

// exerciseSeedRecord defines the JSON structure for an exercise seed item.
type exerciseSeedRecord struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	BodyPartID         string   `json:"body_part_id"`
	EquipmentID        string   `json:"equipment_id"`
	TargetMuscleID     string   `json:"target_muscle_id"`
	SecondaryMuscleIDs []string `json:"secondary_muscle_ids"`
	Instructions       string   `json:"instructions"`
	ThumbnailURL       string   `json:"thumbnail_url"`
	MediaURL           string   `json:"media_url"`
	VideoURL           string   `json:"video_url"`
	Difficulty         string   `json:"difficulty"`
	DefaultRestSeconds int32    `json:"default_rest_seconds"`
	TagIDs             []string `json:"tag_ids"`
	MeasurementType    string   `json:"measurement_type"`
}

// ExerciseRegistrar defines the interface required to create exercises and catalog metadata.
// Implemented by *exerciseGRPC.ExerciseServer and exercisev1service.ExerciseServiceServer.
type ExerciseRegistrar interface {
	CreateExercise(ctx context.Context, req *exercisemsg.CreateExerciseRequest) (*exercisemsg.CreateExerciseResponse, error)
	CreateBodyPart(ctx context.Context, req *exercisemsg.CreateBodyPartRequest) (*exercisemsg.CreateBodyPartResponse, error)
	CreateEquipment(ctx context.Context, req *exercisemsg.CreateEquipmentRequest) (*exercisemsg.CreateEquipmentResponse, error)
	CreateMuscle(ctx context.Context, req *exercisemsg.CreateMuscleRequest) (*exercisemsg.CreateMuscleResponse, error)
	CreateTag(ctx context.Context, req *exercisemsg.CreateTagRequest) (*exercisemsg.CreateTagResponse, error)
}

// SeedExercises seeds both the catalog metadata and the catalog of exercises via the Exercise Service API:
// 1. Seeds body parts, equipments, muscles, tags via API (or maps existing records) to satisfy Foreign Key constraints.
// 2. Checks if exercises are already populated in exercise.exercises; skips if all exist.
// 3. Pre-loads existing exercise names to ensure idempotent execution.
// 4. Calls exerciseService.CreateExercise with an Admin context and MeasurementType for each record.
func SeedExercises(ctx context.Context, db *sql.DB, exerciseService ExerciseRegistrar) error {
	if exerciseService == nil {
		return errors.New("exerciseService cannot be nil")
	}

	// Chuẩn bị context có quyền Admin để vượt qua middleware.RequireAdmin
	adminCtx := context.WithValue(ctx, middleware.UserIDKey, "system-seeder-admin")
	adminCtx = context.WithValue(adminCtx, middleware.UserRoleKey, "Admin")

	// 1. Seed hoặc ánh xạ Metadata danh mục (Body Parts, Equipments, Muscles, Tags) qua API
	idMap, err := seedCatalogMetadata(adminCtx, db, exerciseService)
	if err != nil {
		return fmt.Errorf("seed catalog metadata: %w", err)
	}

	var records []exerciseSeedRecord
	if err := json.Unmarshal(exercisesSeedJSON, &records); err != nil {
		return fmt.Errorf("unmarshal exercise seed json: %w", err)
	}

	totalRecords := len(records)
	if totalRecords == 0 {
		slog.InfoContext(ctx, "No exercise seed records found. Skipping")
		return nil
	}

	// 2. Kiểm tra nhanh: nếu số lượng bài tập trong DB đã đầy đủ thì bỏ qua
	existingNames := make(map[string]bool, totalRecords)
	if db != nil {
		var currentCount int
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM exercise.exercises`).Scan(&currentCount)
		if err == nil && currentCount >= totalRecords {
			slog.InfoContext(ctx, "Exercise catalog already fully seeded, skipping exercise seeder",
				slog.Int("current_count", currentCount),
			)
			return nil
		}

		// Nạp danh sách tên bài tập hiện có vào map để kiểm tra trùng lặp
		rows, err := db.QueryContext(ctx, `SELECT LOWER(name) FROM exercise.exercises`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				if scanErr := rows.Scan(&name); scanErr == nil {
					existingNames[name] = true
				}
			}
		}
	}

	slog.InfoContext(ctx, "Starting Exercise catalog seeding",
		slog.Int("total_records", totalRecords),
		slog.Int("already_existing", len(existingNames)),
	)

	createdCount := 0
	for i := range records {
		record := &records[i]
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		trimmedName := strings.TrimSpace(record.Name)
		if existingNames[strings.ToLower(trimmedName)] {
			continue
		}

		mType := exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS
		if strings.EqualFold(record.MeasurementType, "DURATION") {
			mType = exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION
		}

		// Ánh xạ sang ID mới từ Metadata Seeder
		bodyPartID := idMap[record.BodyPartID]
		if bodyPartID == "" {
			bodyPartID = record.BodyPartID
		}

		equipmentID := idMap[record.EquipmentID]
		if equipmentID == "" {
			equipmentID = record.EquipmentID
		}

		targetMuscleID := idMap[record.TargetMuscleID]
		if targetMuscleID == "" {
			targetMuscleID = record.TargetMuscleID
		}

		var secondaryMuscleIDs []string
		for _, smID := range record.SecondaryMuscleIDs {
			mappedID := idMap[smID]
			if mappedID != "" {
				secondaryMuscleIDs = append(secondaryMuscleIDs, mappedID)
			} else if smID != "" {
				secondaryMuscleIDs = append(secondaryMuscleIDs, smID)
			}
		}

		var tagIDs []string
		for _, tID := range record.TagIDs {
			mappedID := idMap[tID]
			if mappedID != "" {
				tagIDs = append(tagIDs, mappedID)
			} else if tID != "" {
				tagIDs = append(tagIDs, tID)
			}
		}

		req := &exercisemsg.CreateExerciseRequest{
			Name:               trimmedName,
			BodyPartId:         bodyPartID,
			EquipmentId:        equipmentID,
			TargetMuscleId:     targetMuscleID,
			Instructions:       record.Instructions,
			SecondaryMuscleIds: secondaryMuscleIDs,
			ThumbnailUrl:       record.ThumbnailURL,
			MediaUrl:           record.MediaURL,
			VideoUrl:           record.VideoURL,
			Difficulty:         record.Difficulty,
			DefaultRestSeconds: record.DefaultRestSeconds,
			TagIds:             tagIDs,
			MeasurementType:    mType,
			Status:             exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE,
		}

		_, err := exerciseService.CreateExercise(adminCtx, req)
		if err != nil {
			slog.WarnContext(adminCtx, "Failed to seed exercise",
				slog.String("name", record.Name),
				slog.String("error", err.Error()),
			)
			continue
		}

		existingNames[strings.ToLower(trimmedName)] = true
		createdCount++

		// Throttling nhẹ nhàng sau mỗi 25 bài để giải phóng scheduler và tránh nghẽn connection pool
		if createdCount%25 == 0 {
			time.Sleep(5 * time.Millisecond)
		}

		// Log tiến độ định kỳ mỗi 200 bài
		if (i+1)%200 == 0 || i+1 == totalRecords {
			slog.InfoContext(ctx, "Seeding exercises progress",
				slog.Int("progress", i+1),
				slog.Int("total", totalRecords),
				slog.Int("newly_inserted", createdCount),
			)
		}
	}

	slog.InfoContext(ctx, "Completed Exercise catalog seeding",
		slog.Int("new_exercises_created", createdCount),
		slog.Int("total_catalog", len(existingNames)),
	)
	return nil
}

// seedCatalogMetadata ensures body parts, equipments, muscles, and tags are populated via API,
// returning a map of [oldSeedUUID] => [liveDatabaseUUID].
func seedCatalogMetadata(ctx context.Context, db *sql.DB, registrar ExerciseRegistrar) (map[string]string, error) {
	var meta catalogMetadataSeed
	if err := json.Unmarshal(catalogMetadataSeedJSON, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal catalog metadata json: %w", err)
	}

	idMap := make(map[string]string)

	existingBodyParts := queryExistingMap(ctx, db, "exercise.body_parts")
	existingEquipments := queryExistingMap(ctx, db, "exercise.equipments")
	existingMuscles := queryExistingMap(ctx, db, "exercise.muscles")
	existingTags := queryExistingMap(ctx, db, "exercise.tags")

	// 1. Seed Body Parts
	for _, bp := range meta.BodyParts {
		lowerName := strings.ToLower(strings.TrimSpace(bp.Name))
		if liveID, ok := existingBodyParts[lowerName]; ok {
			idMap[bp.ID] = liveID
			continue
		}

		resp, err := registrar.CreateBodyPart(ctx, &exercisemsg.CreateBodyPartRequest{Name: bp.Name})
		if err != nil {
			return nil, fmt.Errorf("create body part %q via API: %w", bp.Name, err)
		}
		idMap[bp.ID] = resp.BodyPart.Id
		existingBodyParts[lowerName] = resp.BodyPart.Id
	}

	// 2. Seed Equipments
	for _, eq := range meta.Equipments {
		lowerName := strings.ToLower(strings.TrimSpace(eq.Name))
		if liveID, ok := existingEquipments[lowerName]; ok {
			idMap[eq.ID] = liveID
			continue
		}

		resp, err := registrar.CreateEquipment(ctx, &exercisemsg.CreateEquipmentRequest{Name: eq.Name})
		if err != nil {
			return nil, fmt.Errorf("create equipment %q via API: %w", eq.Name, err)
		}
		idMap[eq.ID] = resp.Equipment.Id
		existingEquipments[lowerName] = resp.Equipment.Id
	}

	// 3. Seed Muscles (depends on Body Parts)
	for _, m := range meta.Muscles {
		lowerName := strings.ToLower(strings.TrimSpace(m.Name))
		if liveID, ok := existingMuscles[lowerName]; ok {
			idMap[m.ID] = liveID
			continue
		}

		targetBPID := idMap[m.BodyPartID]
		if targetBPID == "" {
			targetBPID = m.BodyPartID
		}

		resp, err := registrar.CreateMuscle(ctx, &exercisemsg.CreateMuscleRequest{
			Name:       m.Name,
			BodyPartId: targetBPID,
		})
		if err != nil {
			return nil, fmt.Errorf("create muscle %q via API: %w", m.Name, err)
		}
		idMap[m.ID] = resp.Muscle.Id
		existingMuscles[lowerName] = resp.Muscle.Id
	}

	// 4. Seed Tags
	for _, t := range meta.Tags {
		lowerName := strings.ToLower(strings.TrimSpace(t.Name))
		if liveID, ok := existingTags[lowerName]; ok {
			idMap[t.ID] = liveID
			continue
		}

		resp, err := registrar.CreateTag(ctx, &exercisemsg.CreateTagRequest{Name: t.Name})
		if err != nil {
			return nil, fmt.Errorf("create tag %q via API: %w", t.Name, err)
		}
		idMap[t.ID] = resp.Tag.Id
		existingTags[lowerName] = resp.Tag.Id
	}

	return idMap, nil
}

// queryExistingMap retrieves an existing [lowerName] => [id] map from a lookup table.
func queryExistingMap(ctx context.Context, db *sql.DB, tableName string) map[string]string {
	result := make(map[string]string)
	if db == nil {
		return result
	}

	query := "SELECT id, LOWER(name) FROM " + tableName
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err == nil {
			result[name] = id
		}
	}

	return result
}
