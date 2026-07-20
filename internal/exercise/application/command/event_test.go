package command

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type mockIDGenerator struct {
	id  string
	err error
}

func (m mockIDGenerator) NewID() (string, error) {
	return m.id, m.err
}

func TestNewEvent_CloudEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	info := domain.Info{
		ID:                 "exercise-123",
		Name:               "Bench Press",
		BodyPartID:         "chest",
		EquipmentID:        "barbell",
		TargetMuscleID:     "pecs",
		SecondaryMuscleIDs: []string{"triceps"},
		Instructions:       "Lie down and press.",
		ThumbnailURL:       "thumb.png",
		MediaURL:           "media.png",
		VideoURL:           "video.mp4",
		Difficulty:         "Intermediate",
		DefaultRestSeconds: 90,
		TagIDs:             []string{"power"},
		Status:             domain.StatusDraft,
	}

	exercise, err := domain.NewExercise(info, now)
	if err != nil {
		t.Fatalf("failed to create exercise: %v", err)
	}

	tests := []struct {
		name       string
		generator  mockIDGenerator
		eventType  string
		wantErrIs  error
		expectType string
	}{
		{
			name: "success created event",
			generator: mockIDGenerator{
				id: "event-uuid-789",
			},
			eventType:  domain.EventTypeExerciseCreated,
			expectType: "contracts.supporting.exercise.v1.exerciseCreated",
		},
		{
			name: "success approved event",
			generator: mockIDGenerator{
				id: "event-uuid-abc",
			},
			eventType:  domain.EventTypeExerciseApproved,
			expectType: "contracts.supporting.exercise.v1.exerciseApproved",
		},
		{
			name: "generator error",
			generator: mockIDGenerator{
				err: errors.New("id generator error"),
			},
			eventType: domain.EventTypeExerciseCreated,
			wantErrIs: domain.ErrInvalidOutboxPayload,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			event, createErr := newEvent(tt.generator, tt.eventType, exercise, now)
			if tt.wantErrIs != nil {
				if createErr == nil {
					t.Fatalf("got nil error, want error")
				}
				return
			}

			if createErr != nil {
				t.Fatalf("got error %v, want nil", createErr)
			}

			if got := event.ID; got != tt.generator.id {
				t.Errorf("got event ID %q, want %q", got, tt.generator.id)
			}
			if got := event.Type; got != tt.eventType {
				t.Errorf("got event type %q, want %q", got, tt.eventType)
			}
			if got := event.PartitionKey; got != info.ID {
				t.Errorf("got event partition key %q, want %q", got, info.ID)
			}

			// Verify CloudEvents payload JSON structure
			var envelope map[string]any
			if unmarshalErr := json.Unmarshal(event.Payload, &envelope); unmarshalErr != nil {
				t.Fatalf("failed to unmarshal envelope: %v", unmarshalErr)
			}

			if got := envelope["specversion"]; got != "1.0" {
				t.Errorf("got specversion %v, want 1.0", got)
			}
			if got := envelope["id"]; got != tt.generator.id {
				t.Errorf("got ID %v, want %s", got, tt.generator.id)
			}
			if got := envelope["source"]; got != "services/exercise-service" {
				t.Errorf("got source %v, want services/exercise-service", got)
			}
			if got := envelope["type"]; got != tt.expectType {
				t.Errorf("got type %v, want %s", got, tt.expectType)
			}
			if got := envelope["time"]; got != now.Format(time.RFC3339) {
				t.Errorf("got time %v, want %s", got, now.Format(time.RFC3339))
			}
			if got := envelope["datacontenttype"]; got != "application/json" {
				t.Errorf("got datacontenttype %v, want application/json", got)
			}

			// Verify nested data block
			dataRaw, ok := envelope["data"]
			if !ok {
				t.Fatalf("missing data block in envelope")
			}

			dataBytes, marshalErr := json.Marshal(dataRaw)
			if marshalErr != nil {
				t.Fatalf("failed to marshal data block: %v", marshalErr)
			}

			var data map[string]any
			if unmarshalErr := json.Unmarshal(dataBytes, &data); unmarshalErr != nil {
				t.Fatalf("failed to unmarshal data block: %v", unmarshalErr)
			}

			exerciseIDVal, ok := data["exerciseId"]
			if !ok {
				t.Fatalf("missing exerciseId inside data payload")
			}
			exerciseIDStr, ok := exerciseIDVal.(string)
			if !ok {
				t.Fatalf("exerciseId is not a string")
			}
			if exerciseIDStr != info.ID {
				t.Errorf("got exerciseId %v, want %s", exerciseIDStr, info.ID)
			}

			exerciseVal, ok := data["exercise"]
			if !ok {
				t.Fatalf("missing exercise info inside data payload")
			}

			exerciseMap, ok := exerciseVal.(map[string]any)
			if !ok {
				t.Fatalf("exercise is not a map[string]any")
			}
			if got := exerciseMap["id"]; got != info.ID {
				t.Errorf("got nested exercise id %v, want %s", got, info.ID)
			}
			if got := exerciseMap["name"]; got != info.Name {
				t.Errorf("got nested exercise name %v, want %s", got, info.Name)
			}
			if got := exerciseMap["bodyPartId"]; got != info.BodyPartID {
				t.Errorf("got nested exercise bodyPartId %v, want %s", got, info.BodyPartID)
			}
			if got := exerciseMap["status"]; got != "EXERCISE_STATUS_DRAFT" {
				t.Errorf("got nested exercise status %v, want EXERCISE_STATUS_DRAFT", got)
			}
		})
	}
}
