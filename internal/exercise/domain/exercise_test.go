package domain

import (
	"errors"
	"testing"
	"time"
)

func TestExerciseTransitions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	exercise := newValidExercise(t, now)

	if got := exercise.Info().Status; got != StatusDraft {
		t.Fatalf("got status %q, want %q", got, StatusDraft)
	}

	if err := exercise.SubmitForApproval(now.Add(time.Minute)); err != nil {
		t.Fatalf("submit for approval: %v", err)
	}
	if got := exercise.Info().Status; got != StatusPendingApproval {
		t.Fatalf("got status %q, want %q", got, StatusPendingApproval)
	}

	if err := exercise.Approve(now.Add(2 * time.Minute)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got := exercise.Info().Status; got != StatusActive {
		t.Fatalf("got status %q, want %q", got, StatusActive)
	}

	if err := exercise.Archive(now.Add(3 * time.Minute)); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if got := exercise.Info().Status; got != StatusArchived {
		t.Fatalf("got status %q, want %q", got, StatusArchived)
	}
}

func TestExerciseInvalidTransitions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		give func(*Exercise) error
	}{
		{
			name: "approve active",
			give: func(exercise *Exercise) error {
				if err := exercise.Approve(now); err != nil {
					return err
				}
				return exercise.Approve(now)
			},
		},
		{
			name: "restore draft",
			give: func(exercise *Exercise) error {
				return exercise.Restore(now)
			},
		},
		{
			name: "restore active",
			give: func(exercise *Exercise) error {
				if err := exercise.Approve(now); err != nil {
					return err
				}
				return exercise.Restore(now)
			},
		},
		{
			name: "submit active",
			give: func(exercise *Exercise) error {
				if err := exercise.SubmitForApproval(now); err != nil {
					return err
				}
				if err := exercise.Approve(now); err != nil {
					return err
				}

				return exercise.SubmitForApproval(now)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			exercise := newValidExercise(t, now)
			err := tt.give(exercise)
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("got error %v, want %v", err, ErrInvalidTransition)
			}
		})
	}
}

func TestArchivedExerciseCannotBeUpdated(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	exercise := newValidExercise(t, now)
	if err := exercise.Archive(now); err != nil {
		t.Fatalf("archive: %v", err)
	}

	info := validInfo()
	info.Name = "Updated"
	err := exercise.UpdateInfo(info, now)
	if !errors.Is(err, ErrArchivedExercise) {
		t.Fatalf("got error %v, want %v", err, ErrArchivedExercise)
	}
}

func TestRehydrateRejectsUnspecifiedStatus(t *testing.T) {
	t.Parallel()
	info := validInfo()
	info.Status = ""

	_, err := RehydrateExercise(info)
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("got error %v, want %v", err, ErrInvalidStatus)
	}
}

func newValidExercise(t *testing.T, now time.Time) *Exercise {
	t.Helper()

	exercise, err := NewExercise(validInfo(), now)
	if err != nil {
		t.Fatalf("new exercise: %v", err)
	}

	return exercise
}

func validInfo() Info {
	return Info{
		ID:             "exercise-1",
		Name:           "Squat",
		BodyPartID:     "legs",
		EquipmentID:    "barbell",
		TargetMuscleID: "quads",
	}
}

func TestNewExercise_Validations(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		give Info
		want error
	}{
		{
			name: "missing id",
			give: Info{
				ID:             "",
				Name:           "Squat",
				BodyPartID:     "legs",
				EquipmentID:    "barbell",
				TargetMuscleID: "quads",
			},
			want: ErrInvalidExercise,
		},
		{
			name: "missing name",
			give: Info{
				ID:             "exercise-1",
				Name:           "",
				BodyPartID:     "legs",
				EquipmentID:    "barbell",
				TargetMuscleID: "quads",
			},
			want: ErrInvalidExercise,
		},
		{
			name: "missing body part id",
			give: Info{
				ID:             "exercise-1",
				Name:           "Squat",
				BodyPartID:     "",
				EquipmentID:    "barbell",
				TargetMuscleID: "quads",
			},
			want: ErrInvalidExercise,
		},
		{
			name: "missing equipment id",
			give: Info{
				ID:             "exercise-1",
				Name:           "Squat",
				BodyPartID:     "legs",
				EquipmentID:    "",
				TargetMuscleID: "quads",
			},
			want: ErrInvalidExercise,
		},
		{
			name: "missing target muscle id",
			give: Info{
				ID:             "exercise-1",
				Name:           "Squat",
				BodyPartID:     "legs",
				EquipmentID:    "barbell",
				TargetMuscleID: "",
			},
			want: ErrInvalidExercise,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewExercise(tt.give, now)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got error %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewExercise_Normalization(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	give := Info{
		ID:                 "  exercise-1  ",
		Name:               "  Squat  ",
		BodyPartID:         "  legs  ",
		EquipmentID:        "  barbell  ",
		TargetMuscleID:     "  quads  ",
		Difficulty:         "",
		DefaultRestSeconds: 0,
	}

	exercise, err := NewExercise(give, now)
	if err != nil {
		t.Fatalf("new exercise: %v", err)
	}

	info := exercise.Info()
	if got, want := info.ID, "exercise-1"; got != want {
		t.Errorf("got id %q, want %q", got, want)
	}
	if got, want := info.Name, "Squat"; got != want {
		t.Errorf("got name %q, want %q", got, want)
	}
	if got, want := info.Difficulty, "Beginner"; got != want {
		t.Errorf("got difficulty %q, want %q", got, want)
	}
	if got, want := info.DefaultRestSeconds, int32(60); got != want {
		t.Errorf("got default rest seconds %d, want %d", got, want)
	}
	if got, want := info.MeasurementType, MeasurementTypeReps; got != want {
		t.Errorf("got measurement type %q, want %q", got, want)
	}
}

func TestUpdateInfo_Validations(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		give Info
		want error
	}{
		{
			name: "missing name",
			give: Info{
				Name:           "",
				BodyPartID:     "legs",
				EquipmentID:    "barbell",
				TargetMuscleID: "quads",
			},
			want: ErrInvalidExercise,
		},
		{
			name: "missing body part id",
			give: Info{
				Name:           "Squat",
				BodyPartID:     "",
				EquipmentID:    "barbell",
				TargetMuscleID: "quads",
			},
			want: ErrInvalidExercise,
		},
		{
			name: "invalid measurement type",
			give: Info{
				Name:            "Squat",
				BodyPartID:      "legs",
				EquipmentID:     "barbell",
				TargetMuscleID:  "quads",
				MeasurementType: "INVALID",
			},
			want: ErrInvalidExercise,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			exercise := newValidExercise(t, now)
			err := exercise.UpdateInfo(tt.give, now)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got error %v, want %v", err, tt.want)
			}
		})
	}
}

func TestMeasurementType(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	t.Run("default to reps", func(t *testing.T) {
		t.Parallel()
		info := validInfo()
		info.MeasurementType = ""
		exercise, err := NewExercise(info, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := exercise.Info().MeasurementType, MeasurementTypeReps; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("explicit duration type", func(t *testing.T) {
		t.Parallel()
		info := validInfo()
		info.MeasurementType = MeasurementTypeDuration
		exercise, err := NewExercise(info, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := exercise.Info().MeasurementType, MeasurementTypeDuration; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("update measurement type", func(t *testing.T) {
		t.Parallel()
		exercise := newValidExercise(t, now)
		info := exercise.Info()
		info.MeasurementType = MeasurementTypeDuration
		if err := exercise.UpdateInfo(info, now.Add(time.Minute)); err != nil {
			t.Fatalf("update info: %v", err)
		}
		if got, want := exercise.Info().MeasurementType, MeasurementTypeDuration; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("invalid measurement type on create", func(t *testing.T) {
		t.Parallel()
		info := validInfo()
		info.MeasurementType = "UNRECOGNIZED"
		_, err := NewExercise(info, now)
		if !errors.Is(err, ErrInvalidExercise) {
			t.Fatalf("got error %v, want %v", err, ErrInvalidExercise)
		}
	})
}

func TestExerciseRestoreCycle(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	exercise := newValidExercise(t, now)

	// 1. Direct approve from Draft to Active
	if err := exercise.Approve(now); err != nil {
		t.Fatalf("direct approve from draft: %v", err)
	}
	if got := exercise.Info().Status; got != StatusActive {
		t.Fatalf("got status %q, want %q", got, StatusActive)
	}

	// 2. Archive Active exercise
	archiveTime := now.Add(time.Minute)
	if err := exercise.Archive(archiveTime); err != nil {
		t.Fatalf("archive active: %v", err)
	}
	if got := exercise.Info().Status; got != StatusArchived {
		t.Fatalf("got status %q, want %q", got, StatusArchived)
	}
	if exercise.Info().ArchivedAt == nil || !exercise.Info().ArchivedAt.Equal(archiveTime) {
		t.Fatalf("expected archived_at %v, got %v", archiveTime, exercise.Info().ArchivedAt)
	}

	// 3. Restore Archived exercise back to Active
	restoreTime := now.Add(2 * time.Minute)
	if err := exercise.Restore(restoreTime); err != nil {
		t.Fatalf("restore archived: %v", err)
	}
	if got := exercise.Info().Status; got != StatusActive {
		t.Fatalf("got status %q, want %q", got, StatusActive)
	}
	if exercise.Info().ArchivedAt != nil {
		t.Fatalf("expected archived_at nil, got %v", exercise.Info().ArchivedAt)
	}
	if !exercise.Info().UpdatedAt.Equal(restoreTime) {
		t.Fatalf("expected updated_at %v, got %v", restoreTime, exercise.Info().UpdatedAt)
	}
}
