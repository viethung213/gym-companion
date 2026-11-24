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
			name: "approve archived",
			give: func(exercise *Exercise) error {
				if err := exercise.Archive(now); err != nil {
					return err
				}
				return exercise.Approve(now)
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
		ID:            "exercise-1",
		Slug:          "Barbell_Squat",
		Name:          "Squat",
		PrimaryMuscle: MuscleGroupQuadriceps,
		Equipment:     EquipmentBarbell,
		Category:      CategoryStrength,
		Level:         LevelBeginner,
		Source:        SourceFreeExerciseDB,
	}
}

func TestNewExercise_Validations(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		give func(*Info)
		want error
	}{
		{name: "missing id", give: func(i *Info) { i.ID = "" }, want: ErrInvalidExercise},
		{name: "missing slug", give: func(i *Info) { i.Slug = " " }, want: ErrInvalidExercise},
		{name: "missing name", give: func(i *Info) { i.Name = "" }, want: ErrInvalidExercise},
		{name: "unknown source", give: func(i *Info) { i.Source = "wger" }, want: ErrInvalidExercise},
		{name: "missing primary muscle", give: func(i *Info) { i.PrimaryMuscle = "" }, want: ErrInvalidExercise},
		{name: "unknown primary muscle", give: func(i *Info) { i.PrimaryMuscle = "QUADS" }, want: ErrInvalidExercise},
		{
			name: "unknown secondary muscle",
			give: func(i *Info) { i.SecondaryMuscles = []MuscleGroup{MuscleGroupGlutes, "Upper Legs"} },
			want: ErrInvalidExercise,
		},
		{name: "missing equipment", give: func(i *Info) { i.Equipment = "" }, want: ErrInvalidExercise},
		{name: "unknown equipment", give: func(i *Info) { i.Equipment = "body weight" }, want: ErrInvalidExercise},
		{name: "missing category", give: func(i *Info) { i.Category = "" }, want: ErrInvalidExercise},
		{name: "missing level", give: func(i *Info) { i.Level = "" }, want: ErrInvalidExercise},
		{name: "advanced is not a level", give: func(i *Info) { i.Level = "ADVANCED" }, want: ErrInvalidExercise},
		{name: "unknown force", give: func(i *Info) { i.Force = "TWIST" }, want: ErrInvalidExercise},
		{name: "unknown mechanic", give: func(i *Info) { i.Mechanic = "HYBRID" }, want: ErrInvalidExercise},
		{
			name: "ai without motion slug",
			give: func(i *Info) { i.HasAISupported = true },
			want: ErrMotionSlugRequired,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := validInfo()
			tt.give(&info)
			_, err := NewExercise(info, now)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got error %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewExercise_OptionalForceAndMechanic(t *testing.T) {
	t.Parallel()
	info := validInfo()
	info.Force = ""
	info.Mechanic = ""

	exercise, err := NewExercise(info, time.Now())
	if err != nil {
		t.Fatalf("new exercise: %v", err)
	}
	if got := exercise.Info(); got.Force != "" || got.Mechanic != "" {
		t.Errorf("got force %q mechanic %q, want empty", got.Force, got.Mechanic)
	}
}

func TestNewExercise_Normalization(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	give := validInfo()
	give.ID = "  exercise-1  "
	give.Name = "  Squat  "
	give.MotionSlug = " squat "
	give.SecondaryMuscles = []MuscleGroup{
		MuscleGroupGlutes, MuscleGroupQuadriceps, MuscleGroupGlutes, MuscleGroupCalves,
	}
	give.Instructions = []string{" Stand. ", "", "Squat."}
	give.Images = []string{"Barbell_Squat/0.jpg", " "}

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
	if got, want := info.MotionSlug, "squat"; got != want {
		t.Errorf("got motion slug %q, want %q", got, want)
	}
	wantSecondary := []MuscleGroup{MuscleGroupGlutes, MuscleGroupCalves}
	if len(info.SecondaryMuscles) != 2 || info.SecondaryMuscles[0] != wantSecondary[0] ||
		info.SecondaryMuscles[1] != wantSecondary[1] {
		t.Errorf("got secondary %v, want %v (deduped, primary dropped)", info.SecondaryMuscles, wantSecondary)
	}
	if len(info.Instructions) != 2 || info.Instructions[0] != "Stand." {
		t.Errorf("got instructions %q", info.Instructions)
	}
	if len(info.Images) != 1 {
		t.Errorf("got images %q, want one", info.Images)
	}
	if got, want := info.DefaultRestSeconds, int32(60); got != want {
		t.Errorf("got default rest seconds %d, want %d", got, want)
	}
	if got, want := info.MeasurementType, MeasurementTypeReps; got != want {
		t.Errorf("got measurement type %q, want %q", got, want)
	}
}

func TestInfo_ReturnsCopies(t *testing.T) {
	t.Parallel()
	info := validInfo()
	info.SecondaryMuscles = []MuscleGroup{MuscleGroupGlutes}
	info.Images = []string{"a.jpg"}
	exercise, err := NewExercise(info, time.Now())
	if err != nil {
		t.Fatalf("new exercise: %v", err)
	}

	got := exercise.Info()
	got.SecondaryMuscles[0] = MuscleGroupChest
	got.Images[0] = "b.jpg"

	if again := exercise.Info(); again.SecondaryMuscles[0] != MuscleGroupGlutes || again.Images[0] != "a.jpg" {
		t.Errorf("aggregate state changed through Info(): %v %v", again.SecondaryMuscles, again.Images)
	}
}

func TestUpdateInfo_Validations(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		give func(*Info)
		want error
	}{
		{name: "missing name", give: func(i *Info) { i.Name = "" }, want: ErrInvalidExercise},
		{name: "unknown category", give: func(i *Info) { i.Category = "YOGA" }, want: ErrInvalidExercise},
		{
			name: "invalid measurement type",
			give: func(i *Info) { i.MeasurementType = "INVALID" },
			want: ErrInvalidExercise,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			exercise := newValidExercise(t, now)
			info := validInfo()
			tt.give(&info)
			err := exercise.UpdateInfo(info, now)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got error %v, want %v", err, tt.want)
			}
		})
	}
}

func TestUpdateInfo_KeepsIdentityAndAIFlag(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	info := validInfo()
	info.MotionSlug = "squat"
	info.HasAISupported = true
	exercise, err := NewExercise(info, now)
	if err != nil {
		t.Fatalf("new exercise: %v", err)
	}

	update := validInfo()
	update.Slug = "Other_Slug"
	update.Source = SourceCustom
	update.MotionSlug = "squat"
	update.Level = LevelExpert
	if err := exercise.UpdateInfo(update, now); err != nil {
		t.Fatalf("update: %v", err)
	}

	got := exercise.Info()
	if got.Slug != "Barbell_Squat" || got.Source != SourceFreeExerciseDB || !got.HasAISupported {
		t.Errorf("got slug %q source %q ai %v; identity and AI flag must not change",
			got.Slug, got.Source, got.HasAISupported)
	}
	if got.Level != LevelExpert {
		t.Errorf("got level %q, want EXPERT", got.Level)
	}

	update.MotionSlug = ""
	if err := exercise.UpdateInfo(update, now); !errors.Is(err, ErrMotionSlugRequired) {
		t.Errorf("clearing motion slug of an AI exercise: got %v, want %v", err, ErrMotionSlugRequired)
	}
}

func TestSetAISupported(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		motionSlug string
		give       bool
		want       error
	}{
		{name: "enable with motion slug", motionSlug: "squat", give: true},
		{name: "enable without motion slug", give: true, want: ErrMotionSlugRequired},
		{name: "disable without motion slug", give: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := validInfo()
			info.MotionSlug = tt.motionSlug
			exercise, err := NewExercise(info, now)
			if err != nil {
				t.Fatalf("new exercise: %v", err)
			}

			err = exercise.SetAISupported(tt.give, now)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got error %v, want %v", err, tt.want)
			}
			if tt.want == nil && exercise.Info().HasAISupported != tt.give {
				t.Errorf("got has_ai_supported %v, want %v", exercise.Info().HasAISupported, tt.give)
			}
		})
	}
}

func TestSlugFromName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		give string
		want string
	}{
		{give: "Barbell Squat", want: "Barbell_Squat"},
		{give: "3/4 Sit-Up", want: "3_4_Sit-Up"},
		{give: "  Dips - Triceps Version ", want: "Dips_-_Triceps_Version"},
		{give: "Cable (Rope) Pushdown!", want: "Cable_Rope_Pushdown"},
	}

	for _, tt := range tests {
		if got := SlugFromName(tt.give); got != tt.want {
			t.Errorf("SlugFromName(%q) = %q, want %q", tt.give, got, tt.want)
		}
	}
}

func TestParseTaxonomy(t *testing.T) {
	t.Parallel()

	if got, err := ParseMuscleGroup("LOWER_BACK"); err != nil || got != MuscleGroupLowerBack {
		t.Errorf("ParseMuscleGroup(LOWER_BACK) = %q, %v", got, err)
	}
	if got, err := ParseEquipment("EZ_CURL_BAR"); err != nil || got != EquipmentEZCurlBar {
		t.Errorf("ParseEquipment(EZ_CURL_BAR) = %q, %v", got, err)
	}

	invalid := []func() error{
		func() error { _, err := ParseMuscleGroup("lower back"); return err },
		func() error { _, err := ParseEquipment(""); return err },
		func() error { _, err := ParseCategory("YOGA"); return err },
		func() error { _, err := ParseLevel("ADVANCED"); return err },
		func() error { _, err := ParseForce("push"); return err },
		func() error { _, err := ParseMechanic("COMPOUNDS"); return err },
	}
	for i, parse := range invalid {
		if err := parse(); !errors.Is(err, ErrInvalidExercise) {
			t.Errorf("case %d: got %v, want ErrInvalidExercise", i, err)
		}
	}

	if got := len(AllMuscleGroups()); got != 17 {
		t.Errorf("got %d muscle groups, want 17", got)
	}
	if got := len(AllEquipment()); got != 12 {
		t.Errorf("got %d equipment values, want 12", got)
	}
	if got := len(AllCategories()); got != 7 {
		t.Errorf("got %d categories, want 7", got)
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
