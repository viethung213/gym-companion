package application

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

func TestCreateExerciseReturnsDraft(t *testing.T) {
	t.Parallel()
	service, repo := newTestService()
	ctx := adminContext()

	exercise, err := service.CreateExercise(ctx, validInfo())
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	if got := exercise.Info().Status; got != domain.StatusDraft {
		t.Fatalf("got status %q, want %q", got, domain.StatusDraft)
	}
	if got := len(repo.events); got != 1 {
		t.Fatalf("got events %d, want 1", got)
	}
}

func TestUpdateExerciseDoesNotChangeStatus(t *testing.T) {
	t.Parallel()
	service, _ := newTestService()
	ctx := adminContext()

	exercise, err := service.CreateExercise(ctx, validInfo())
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	info := validInfo()
	info.Name = "Updated Squat"
	info.Status = domain.StatusActive
	updated, err := service.UpdateExercise(ctx, exercise.Info().ID, info)
	if err != nil {
		t.Fatalf("update exercise: %v", err)
	}

	if got := updated.Info().Status; got != domain.StatusDraft {
		t.Fatalf("got status %q, want %q", got, domain.StatusDraft)
	}
}

func TestArchiveExercise(t *testing.T) {
	t.Parallel()
	service, _ := newTestService()
	ctx := adminContext()

	exercise, err := service.CreateExercise(ctx, validInfo())
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	if archiveErr := service.ArchiveExercise(ctx, exercise.Info().ID); archiveErr != nil {
		t.Fatalf("archive exercise: %v", archiveErr)
	}

	_, err = service.GetExercise(userContext(), exercise.Info().ID)
	if !errors.Is(err, domain.ErrExerciseNotFound) {
		t.Fatalf("got error %v, want %v", err, domain.ErrExerciseNotFound)
	}
}

func TestSearchAndGetOnlyReturnActiveExercises(t *testing.T) {
	t.Parallel()
	service, _ := newTestService()
	adminCtx := adminContext()
	userCtx := userContext()

	active, err := service.CreateExercise(adminCtx, validInfo())
	if err != nil {
		t.Fatalf("create active exercise: %v", err)
	}
	_, submitErr := service.SubmitExerciseForApproval(adminCtx, active.Info().ID)
	if submitErr != nil {
		t.Fatalf("submit exercise: %v", submitErr)
	}
	_, approveErr := service.ApproveExercise(adminCtx, active.Info().ID)
	if approveErr != nil {
		t.Fatalf("approve exercise: %v", approveErr)
	}

	draft, err := service.CreateExercise(adminCtx, validInfo())
	if err != nil {
		t.Fatalf("create draft exercise: %v", err)
	}

	exercises, err := service.SearchExercises(userCtx, &SearchFilters{})
	if err != nil {
		t.Fatalf("search exercises: %v", err)
	}
	if got := len(exercises); got != 1 {
		t.Fatalf("got exercises %d, want 1", got)
	}

	_, err = service.GetExercise(userCtx, draft.Info().ID)
	if !errors.Is(err, domain.ErrExerciseNotFound) {
		t.Fatalf("got error %v, want %v", err, domain.ErrExerciseNotFound)
	}
}

func TestAdminMutationRejectsNonAdmin(t *testing.T) {
	t.Parallel()
	service, _ := newTestService()

	_, err := service.CreateExercise(userContext(), validInfo())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("got error %v, want %v", err, domain.ErrForbidden)
	}
}

type fakeRepository struct {
	exercises map[string]*domain.Exercise
	events    []*domain.Event
}

func (r *fakeRepository) Save(
	_ context.Context,
	exercise *domain.Exercise,
	event *domain.Event,
) error {
	r.exercises[exercise.Info().ID] = exercise
	if event != nil {
		r.events = append(r.events, event)
	}

	return nil
}

func (r *fakeRepository) FindByID(_ context.Context, id string) (*domain.Exercise, error) {
	exercise, ok := r.exercises[id]
	if !ok {
		return nil, domain.ErrExerciseNotFound
	}

	return exercise, nil
}

func (r *fakeRepository) SearchActive(
	_ context.Context,
	_ *SearchFilters,
) ([]*domain.Exercise, error) {
	var exercises []*domain.Exercise
	for _, exercise := range r.exercises {
		if exercise.Info().Status == domain.StatusActive {
			exercises = append(exercises, exercise)
		}
	}

	return exercises, nil
}

func (r *fakeRepository) GetMetadata(_ context.Context) (Metadata, error) {
	return Metadata{}, nil
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

type sequenceIDGenerator struct {
	next int
}

func (g *sequenceIDGenerator) NewID() (string, error) {
	g.next++

	return "id-" + strconv.Itoa(g.next), nil
}

func newTestService() (*Service, *fakeRepository) {
	repo := &fakeRepository{
		exercises: make(map[string]*domain.Exercise),
	}
	ids := &sequenceIDGenerator{}
	service := NewService(repo, fakeClock{
		now: time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
	}, ids)

	return service, repo
}

func adminContext() context.Context {
	return ContextWithActor(context.Background(), Actor{
		UserID: "admin-1",
		Roles:  []string{"Admin"},
	})
}

func userContext() context.Context {
	return ContextWithActor(context.Background(), Actor{
		UserID: "user-1",
		Roles:  []string{"User"},
	})
}

func validInfo() domain.Info {
	return domain.Info{
		Name:           "Squat",
		BodyPartID:     "legs",
		EquipmentID:    "barbell",
		TargetMuscleID: "quads",
	}
}
