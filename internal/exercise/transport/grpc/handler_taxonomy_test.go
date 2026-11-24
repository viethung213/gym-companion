//go:build unit

package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func validCreateRequest() *exercisemsg.CreateExerciseRequest {
	return &exercisemsg.CreateExerciseRequest{
		Name:             "Barbell Squat",
		PrimaryMuscle:    exercisemsg.MuscleGroup_MUSCLE_GROUP_QUADRICEPS,
		SecondaryMuscles: []exercisemsg.MuscleGroup{exercisemsg.MuscleGroup_MUSCLE_GROUP_GLUTES},
		Equipment:        exercisemsg.Equipment_EQUIPMENT_BARBELL,
		Category:         exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_STRENGTH,
		Level:            exercisemsg.ExerciseLevel_EXERCISE_LEVEL_BEGINNER,
		Images:           []string{"Barbell_Squat/0.jpg"},
	}
}

type recordingRepo struct {
	mockRepoForHandler
	filters *port.SearchFilters
	saved   *domain.Exercise
}

func (r *recordingRepo) Save(_ context.Context, exercise *domain.Exercise, _ *domain.Event) error {
	r.saved = exercise
	return nil
}

func (r *recordingRepo) SearchActive(_ context.Context, f *port.SearchFilters) ([]*domain.Exercise, int32, error) {
	r.filters = f
	return nil, 0, nil
}

func adminCtx() context.Context {
	ctx := context.WithValue(context.Background(), middleware.UserRoleKey, "Admin")
	return context.WithValue(ctx, middleware.UserIDKey, "admin-1")
}

func TestCreateExercise_RejectsInvalidTaxonomy(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*exercisemsg.CreateExerciseRequest)
	}{
		{"unspecified primary muscle", func(r *exercisemsg.CreateExerciseRequest) {
			r.PrimaryMuscle = exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED
		}},
		{"unknown secondary muscle", func(r *exercisemsg.CreateExerciseRequest) {
			r.SecondaryMuscles = []exercisemsg.MuscleGroup{99}
		}},
		{"unspecified equipment", func(r *exercisemsg.CreateExerciseRequest) {
			r.Equipment = exercisemsg.Equipment_EQUIPMENT_UNSPECIFIED
		}},
		{"unspecified category", func(r *exercisemsg.CreateExerciseRequest) {
			r.Category = exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_UNSPECIFIED
		}},
		{"unspecified level", func(r *exercisemsg.CreateExerciseRequest) {
			r.Level = exercisemsg.ExerciseLevel_EXERCISE_LEVEL_UNSPECIFIED
		}},
		{"unknown force", func(r *exercisemsg.CreateExerciseRequest) {
			r.Force = 9
		}},
		{"unknown mechanic", func(r *exercisemsg.CreateExerciseRequest) {
			r.Mechanic = 9
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &recordingRepo{}
			srv := NewExerciseServer(
				command.NewCreateExerciseHandler(repo, &mockClock{}, &mockIDs{id: "id-1"}),
				nil, nil, nil, nil, nil, nil, nil, "",
			)
			req := validCreateRequest()
			tt.mutate(req)

			_, err := srv.CreateExercise(adminCtx(), req)
			if got := status.Code(err); got != codes.InvalidArgument {
				t.Errorf("got code %v (%v), want InvalidArgument", got, err)
			}
			if repo.saved != nil {
				t.Error("invalid exercise was saved")
			}
		})
	}
}

func TestCreateExercise_SourceFromSlug(t *testing.T) {
	tests := []struct {
		name       string
		slug       string
		wantSlug   string
		wantSource domain.Source
	}{
		{"imported", "Barbell_Squat", "Barbell_Squat", domain.SourceFreeExerciseDB},
		{"custom", "", "Barbell_Squat", domain.SourceCustom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &recordingRepo{}
			srv := NewExerciseServer(
				command.NewCreateExerciseHandler(repo, &mockClock{}, &mockIDs{id: "id-1"}),
				nil, nil, nil, nil, nil, nil, nil, "https://cdn.example/exercises/",
			)
			req := validCreateRequest()
			req.Slug = tt.slug

			resp, err := srv.CreateExercise(adminCtx(), req)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if got := repo.saved.Info(); got.Slug != tt.wantSlug || got.Source != tt.wantSource {
				t.Errorf("got slug %q source %q, want %q %q", got.Slug, got.Source, tt.wantSlug, tt.wantSource)
			}
			if got := resp.GetExercise().GetImages(); len(got) != 1 ||
				got[0] != "https://cdn.example/exercises/Barbell_Squat/0.jpg" {
				t.Errorf("got images %v, want absolute media URL", got)
			}
			if got := resp.GetExercise().GetForce(); got != exercisemsg.ExerciseForce_EXERCISE_FORCE_UNSPECIFIED {
				t.Errorf("got force %v, want UNSPECIFIED for none", got)
			}
		})
	}
}

func TestSearchExercises_MapsFilters(t *testing.T) {
	repo := &recordingRepo{}
	srv := NewExerciseServer(nil, nil, nil, nil, nil, nil, nil, query.NewSearchExercisesHandler(repo), "")

	_, err := srv.SearchExercises(context.Background(), &exercisemsg.SearchExercisesRequest{
		PrimaryMuscles: []exercisemsg.MuscleGroup{exercisemsg.MuscleGroup_MUSCLE_GROUP_CHEST},
		ExcludeMuscles: []exercisemsg.MuscleGroup{
			exercisemsg.MuscleGroup_MUSCLE_GROUP_QUADRICEPS, exercisemsg.MuscleGroup_MUSCLE_GROUP_CALVES,
		},
		Equipment:      []exercisemsg.Equipment{exercisemsg.Equipment_EQUIPMENT_CABLE},
		Categories:     []exercisemsg.ExerciseCategory{exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_STRETCHING},
		Levels:         []exercisemsg.ExerciseLevel{exercisemsg.ExerciseLevel_EXERCISE_LEVEL_EXPERT},
		Force:          exercisemsg.ExerciseForce_EXERCISE_FORCE_PULL.Enum(),
		HasAiSupported: proto.Bool(true),
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	f := repo.filters
	if len(f.PrimaryMuscles) != 1 || f.PrimaryMuscles[0] != domain.MuscleGroupChest {
		t.Errorf("primary muscles = %v", f.PrimaryMuscles)
	}
	if len(f.ExcludeMuscles) != 2 || f.ExcludeMuscles[1] != domain.MuscleGroupCalves {
		t.Errorf("exclude muscles = %v", f.ExcludeMuscles)
	}
	if f.Equipment[0] != domain.EquipmentCable || f.Categories[0] != domain.CategoryStretching ||
		f.Levels[0] != domain.LevelExpert {
		t.Errorf("equipment/category/level = %v/%v/%v", f.Equipment, f.Categories, f.Levels)
	}
	if f.Force != domain.ForcePull || f.Mechanic != "" {
		t.Errorf("force/mechanic = %q/%q, want PULL and no mechanic filter", f.Force, f.Mechanic)
	}
	if f.HasAISupported == nil || !*f.HasAISupported {
		t.Errorf("has_ai_supported = %v, want true", f.HasAISupported)
	}
}

func TestSearchExercises_RejectsInvalidFilters(t *testing.T) {
	tests := []struct {
		name string
		give *exercisemsg.SearchExercisesRequest
	}{
		{"unspecified exclude muscle", &exercisemsg.SearchExercisesRequest{
			ExcludeMuscles: []exercisemsg.MuscleGroup{exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED},
		}},
		{"unknown equipment", &exercisemsg.SearchExercisesRequest{
			Equipment: []exercisemsg.Equipment{42},
		}},
		{"explicit unspecified force", &exercisemsg.SearchExercisesRequest{
			Force: exercisemsg.ExerciseForce_EXERCISE_FORCE_UNSPECIFIED.Enum(),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &recordingRepo{}
			srv := NewExerciseServer(nil, nil, nil, nil, nil, nil, nil, query.NewSearchExercisesHandler(repo), "")

			_, err := srv.SearchExercises(context.Background(), tt.give)
			if got := status.Code(err); got != codes.InvalidArgument {
				t.Errorf("got code %v, want InvalidArgument", got)
			}
			if repo.filters != nil {
				t.Error("repository was queried with invalid filters")
			}
		})
	}
}

func TestResolveMediaURL(t *testing.T) {
	tests := []struct {
		base, give, want string
	}{
		{"https://cdn/x/", "Plank/0.jpg", "https://cdn/x/Plank/0.jpg"},
		{"https://cdn/x", "/Plank/0.jpg", "https://cdn/x/Plank/0.jpg"},
		{"https://cdn/x/", "https://other/p.jpg", "https://other/p.jpg"},
		{"", "Plank/0.jpg", "Plank/0.jpg"},
	}

	for _, tt := range tests {
		if got := resolveMediaURL(tt.base, tt.give); got != tt.want {
			t.Errorf("resolveMediaURL(%q, %q) = %q, want %q", tt.base, tt.give, got, tt.want)
		}
	}
}

func TestRpcError_CatalogCodes(t *testing.T) {
	tests := []struct {
		give error
		want codes.Code
	}{
		{domain.ErrDuplicateSlug, codes.AlreadyExists},
		{domain.ErrMotionSlugRequired, codes.FailedPrecondition},
		{errors.Join(domain.ErrInvalidExercise, errors.New("unknown level")), codes.InvalidArgument},
	}

	for _, tt := range tests {
		if got := status.Code(rpcError(tt.give)); got != tt.want {
			t.Errorf("rpcError(%v) = %v, want %v", tt.give, got, tt.want)
		}
	}
}
