package application_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

func TestCreateExerciseReturnsDraft(t *testing.T) {
	t.Parallel()
	suite := newTestSuite()
	ctx := adminContext()

	exercise, err := suite.createHandler.Handle(
		ctx,
		&command.CreateExerciseCommand{Info: validInfo()},
	)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	if got := exercise.Info().Status; got != domain.StatusDraft {
		t.Fatalf("got status %q, want %q", got, domain.StatusDraft)
	}
	if got := len(suite.repo.events); got != 1 {
		t.Fatalf("got events %d, want 1", got)
	}
}

func TestUpdateExerciseDoesNotChangeStatus(t *testing.T) {
	t.Parallel()
	suite := newTestSuite()
	ctx := adminContext()

	exercise, err := suite.createHandler.Handle(
		ctx,
		&command.CreateExerciseCommand{Info: validInfo()},
	)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	info := validInfo()
	info.Name = "Updated Squat"
	info.Status = domain.StatusActive
	updated, err := suite.updateHandler.Handle(ctx, &command.UpdateExerciseCommand{
		ID:   exercise.Info().ID,
		Info: info,
	})
	if err != nil {
		t.Fatalf("update exercise: %v", err)
	}

	if got := updated.Info().Status; got != domain.StatusDraft {
		t.Fatalf("got status %q, want %q", got, domain.StatusDraft)
	}
}

func TestArchiveExercise(t *testing.T) {
	t.Parallel()
	suite := newTestSuite()
	ctx := adminContext()

	exercise, err := suite.createHandler.Handle(
		ctx,
		&command.CreateExerciseCommand{Info: validInfo()},
	)
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	err = suite.archiveHandler.Handle(
		ctx,
		command.ArchiveExerciseCommand{ID: exercise.Info().ID},
	)
	if err != nil {
		t.Fatalf("archive exercise: %v", err)
	}

	_, err = suite.getHandler.Handle(userContext(), query.GetExerciseQuery{ID: exercise.Info().ID})
	if !errors.Is(err, domain.ErrExerciseNotFound) {
		t.Fatalf("got error %v, want %v", err, domain.ErrExerciseNotFound)
	}
}

func TestSearchAndGetOnlyReturnActiveExercises(t *testing.T) {
	t.Parallel()
	suite := newTestSuite()
	adminCtx := adminContext()
	userCtx := userContext()

	active, err := suite.createHandler.Handle(
		adminCtx,
		&command.CreateExerciseCommand{Info: validInfo()},
	)
	if err != nil {
		t.Fatalf("create active exercise: %v", err)
	}
	_, err = suite.submitForApprovalHandler.Handle(
		adminCtx,
		command.SubmitExerciseForApprovalCommand{ID: active.Info().ID},
	)
	if err != nil {
		t.Fatalf("submit exercise: %v", err)
	}
	_, err = suite.approveHandler.Handle(
		adminCtx,
		command.ApproveExerciseCommand{ID: active.Info().ID},
	)
	if err != nil {
		t.Fatalf("approve exercise: %v", err)
	}

	draft, err := suite.createHandler.Handle(
		adminCtx,
		&command.CreateExerciseCommand{Info: validInfo()},
	)
	if err != nil {
		t.Fatalf("create draft exercise: %v", err)
	}

	exercises, err := suite.searchHandler.Handle(
		userCtx,
		query.SearchExercisesQuery{Filters: &port.SearchFilters{}},
	)
	if err != nil {
		t.Fatalf("search exercises: %v", err)
	}
	if got := len(exercises); got != 1 {
		t.Fatalf("got exercises %d, want 1", got)
	}

	_, err = suite.getHandler.Handle(userCtx, query.GetExerciseQuery{ID: draft.Info().ID})
	if !errors.Is(err, domain.ErrExerciseNotFound) {
		t.Fatalf("got error %v, want %v", err, domain.ErrExerciseNotFound)
	}
}

func TestAdminMutationRejectsNonAdmin(t *testing.T) {
	t.Parallel()
	suite := newTestSuite()

	_, err := suite.createHandler.Handle(
		userContext(),
		&command.CreateExerciseCommand{Info: validInfo()},
	)
	if !errors.Is(err, middleware.ErrForbidden) {
		t.Fatalf("got error %v, want %v", err, middleware.ErrForbidden)
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
	_ *port.SearchFilters,
) ([]*domain.Exercise, error) {
	var exercises []*domain.Exercise
	for _, exercise := range r.exercises {
		if exercise.Info().Status == domain.StatusActive {
			exercises = append(exercises, exercise)
		}
	}

	return exercises, nil
}

func (r *fakeRepository) GetMetadata(_ context.Context) (port.Metadata, error) {
	return port.Metadata{}, nil
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

type testSuite struct {
	repo                     *fakeRepository
	clock                    fakeClock
	ids                      *sequenceIDGenerator
	createHandler            *command.CreateExerciseHandler
	updateHandler            *command.UpdateExerciseHandler
	submitForApprovalHandler *command.SubmitExerciseForApprovalHandler
	approveHandler           *command.ApproveExerciseHandler
	archiveHandler           *command.ArchiveExerciseHandler
	getHandler               *query.GetExerciseHandler
	searchHandler            *query.SearchExercisesHandler
	metadataHandler          *query.GetCatalogMetadataHandler
}

func newTestSuite() *testSuite {
	repo := &fakeRepository{
		exercises: make(map[string]*domain.Exercise),
	}
	clock := fakeClock{
		now: time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
	}
	ids := &sequenceIDGenerator{}

	return &testSuite{
		repo:                     repo,
		clock:                    clock,
		ids:                      ids,
		createHandler:            command.NewCreateExerciseHandler(repo, clock, ids),
		updateHandler:            command.NewUpdateExerciseHandler(repo, clock),
		submitForApprovalHandler: command.NewSubmitExerciseForApprovalHandler(repo, clock, ids),
		approveHandler:           command.NewApproveExerciseHandler(repo, clock, ids),
		archiveHandler:           command.NewArchiveExerciseHandler(repo, clock, ids),
		getHandler:               query.NewGetExerciseHandler(repo),
		searchHandler:            query.NewSearchExercisesHandler(repo),
		metadataHandler:          query.NewGetCatalogMetadataHandler(repo),
	}
}

func adminContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.UserIDKey, "admin-1")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "Admin")
	return ctx
}

func userContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.UserIDKey, "user-1")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "User")
	return ctx
}

func validInfo() domain.Info {
	return domain.Info{
		Name:           "Squat",
		BodyPartID:     "legs",
		EquipmentID:    "barbell",
		TargetMuscleID: "quads",
	}
}
