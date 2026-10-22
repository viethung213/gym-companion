package persistence

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

func TestMappingExerciseRecord(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	domainInfo := domain.Info{
		ID:                 "ex-123",
		Name:               "Plank",
		BodyPartID:         "waist",
		EquipmentID:        "body-weight",
		TargetMuscleID:     "abs",
		Instructions:       "Hold position",
		SecondaryMuscleIDs: []string{"glutes", "lower-back"},
		ThumbnailURL:       "https://example.com/thumb.jpg",
		MediaURL:           "https://example.com/media.gif",
		VideoURL:           "https://example.com/video.mp4",
		Difficulty:         "Intermediate",
		DefaultRestSeconds: 45,
		TagIDs:             []string{"tag-1", "tag-2"},
		Status:             domain.StatusActive,
		HasAISupported:     true,
		MeasurementType:    domain.MeasurementTypeDuration,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	record := newExerciseRecord(&domainInfo)

	if got, want := record.ID, domainInfo.ID; got != want {
		t.Errorf("got ID %q, want %q", got, want)
	}
	if got, want := record.MeasurementType, "DURATION"; got != want {
		t.Errorf("got MeasurementType %q, want %q", got, want)
	}
	if got, want := record.Status, "ACTIVE"; got != want {
		t.Errorf("got Status %q, want %q", got, want)
	}

	mappedBack := record.toDomainInfo(domainInfo.SecondaryMuscleIDs, domainInfo.TagIDs)

	if got, want := mappedBack.MeasurementType, domain.MeasurementTypeDuration; got != want {
		t.Errorf("got MeasurementType %q, want %q", got, want)
	}
	if got, want := mappedBack.Name, domainInfo.Name; got != want {
		t.Errorf("got Name %q, want %q", got, want)
	}
	if got, want := mappedBack.HasAISupported, true; got != want {
		t.Errorf("got HasAISupported %v, want %v", got, want)
	}
}
