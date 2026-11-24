package persistence

import (
	"github.com/lib/pq"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

func newExerciseRecord(info *domain.Info) exerciseRecord {
	return exerciseRecord{
		ID:                 info.ID,
		Slug:               info.Slug,
		Name:               info.Name,
		PrimaryMuscle:      string(info.PrimaryMuscle),
		SecondaryMuscles:   toTextArray(info.SecondaryMuscles),
		Equipment:          string(info.Equipment),
		Category:           string(info.Category),
		Level:              string(info.Level),
		Force:              optionalString(string(info.Force)),
		Mechanic:           optionalString(string(info.Mechanic)),
		Instructions:       toTextArray(info.Instructions),
		Images:             toTextArray(info.Images),
		VideoURL:           optionalString(info.VideoURL),
		MeasurementType:    string(info.MeasurementType),
		DefaultRestSeconds: info.DefaultRestSeconds,
		MotionSlug:         optionalString(info.MotionSlug),
		HasAISupported:     info.HasAISupported,
		Source:             string(info.Source),
		Status:             string(info.Status),
		ArchivedAt:         info.ArchivedAt,
		CreatedAt:          info.CreatedAt,
		UpdatedAt:          info.UpdatedAt,
	}
}

func (r *exerciseRecord) toDomainInfo() domain.Info {
	return domain.Info{
		ID:                 r.ID,
		Slug:               r.Slug,
		Name:               r.Name,
		PrimaryMuscle:      domain.MuscleGroup(r.PrimaryMuscle),
		SecondaryMuscles:   fromTextArray[domain.MuscleGroup](r.SecondaryMuscles),
		Equipment:          domain.Equipment(r.Equipment),
		Category:           domain.Category(r.Category),
		Level:              domain.Level(r.Level),
		Force:              domain.Force(stringFromPointer(r.Force)),
		Mechanic:           domain.Mechanic(stringFromPointer(r.Mechanic)),
		Instructions:       fromTextArray[string](r.Instructions),
		Images:             fromTextArray[string](r.Images),
		VideoURL:           stringFromPointer(r.VideoURL),
		MeasurementType:    domain.MeasurementType(r.MeasurementType),
		DefaultRestSeconds: r.DefaultRestSeconds,
		MotionSlug:         stringFromPointer(r.MotionSlug),
		HasAISupported:     r.HasAISupported,
		Source:             domain.Source(r.Source),
		Status:             domain.Status(r.Status),
		ArchivedAt:         r.ArchivedAt,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

// toTextArray never returns nil: the TEXT[] columns are NOT NULL.
func toTextArray[T ~string](values []T) pq.StringArray {
	out := make(pq.StringArray, len(values))
	for i, v := range values {
		out[i] = string(v)
	}

	return out
}

func fromTextArray[T ~string](values pq.StringArray) []T {
	if len(values) == 0 {
		return nil
	}
	out := make([]T, len(values))
	for i, v := range values {
		out[i] = T(v)
	}

	return out
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
