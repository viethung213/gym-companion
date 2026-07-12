package persistence

import "github.com/viethung213/gym-companion/internal/exercise/domain"

func newExerciseRecord(info *domain.Info) exerciseRecord {
	return exerciseRecord{
		ID:                 info.ID,
		Name:               info.Name,
		BodyPartID:         info.BodyPartID,
		EquipmentID:        info.EquipmentID,
		TargetMuscleID:     info.TargetMuscleID,
		Instructions:       info.Instructions,
		ThumbnailURL:       optionalString(info.ThumbnailURL),
		MediaURL:           optionalString(info.MediaURL),
		VideoURL:           optionalString(info.VideoURL),
		Difficulty:         info.Difficulty,
		DefaultRestSeconds: info.DefaultRestSeconds,
		Status:             string(info.Status),
		ArchivedAt:         info.ArchivedAt,
		CreatedAt:          info.CreatedAt,
		UpdatedAt:          info.UpdatedAt,
	}
}

func (r *exerciseRecord) toDomainInfo(secondaryMuscleIDs, tagIDs []string) domain.Info {
	return domain.Info{
		ID:                 r.ID,
		Name:               r.Name,
		BodyPartID:         r.BodyPartID,
		EquipmentID:        r.EquipmentID,
		TargetMuscleID:     r.TargetMuscleID,
		SecondaryMuscleIDs: secondaryMuscleIDs,
		TagIDs:             tagIDs,
		Instructions:       r.Instructions,
		ThumbnailURL:       stringFromPointer(r.ThumbnailURL),
		MediaURL:           stringFromPointer(r.MediaURL),
		VideoURL:           stringFromPointer(r.VideoURL),
		Difficulty:         r.Difficulty,
		DefaultRestSeconds: r.DefaultRestSeconds,
		Status:             domain.Status(r.Status),
		ArchivedAt:         r.ArchivedAt,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

func newSecondaryMuscleRecords(
	exerciseID string,
	muscleIDs []string,
) []exerciseSecondaryMuscleRecord {
	records := make([]exerciseSecondaryMuscleRecord, 0, len(muscleIDs))
	for _, muscleID := range muscleIDs {
		records = append(records, exerciseSecondaryMuscleRecord{
			ExerciseID: exerciseID,
			MuscleID:   muscleID,
		})
	}

	return records
}

func newTagRecords(exerciseID string, tagIDs []string) []exerciseTagRecord {
	records := make([]exerciseTagRecord, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		records = append(records, exerciseTagRecord{
			ExerciseID: exerciseID,
			TagID:      tagID,
		})
	}

	return records
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	copyValue := value
	return &copyValue
}

func stringFromPointer(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
