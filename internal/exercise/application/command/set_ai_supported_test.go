//go:build unit

package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application/command"
	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

type mockClock struct {
	now time.Time
}

func (m mockClock) Now() time.Time {
	return m.now
}

type mockExerciseRepo struct {
	port.Repository
	exercise  *domain.Exercise
	saved     *domain.Exercise
	loggedFor string
	err       error
}

func (m *mockExerciseRepo) FindByID(_ context.Context, _ string) (*domain.Exercise, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.exercise == nil {
		return nil, domain.ErrExerciseNotFound
	}
	return m.exercise, nil
}

func (m *mockExerciseRepo) Save(_ context.Context, exercise *domain.Exercise, _ *domain.Event) error {
	m.saved = exercise
	return nil
}

func (m *mockExerciseRepo) SetAISupportedWithOutboxLog(_ context.Context, exerciseID string, _ bool, _ *port.OutboxLogRecord) error {
	m.loggedFor = exerciseID
	return nil
}

func newExercise(t *testing.T, motionSlug string) *domain.Exercise {
	t.Helper()

	ex, err := domain.NewExercise(domain.Info{
		ID:            "ex-1",
		Slug:          "Barbell_Bench_Press_-_Medium_Grip",
		Name:          "Bench Press",
		PrimaryMuscle: domain.MuscleGroupChest,
		Equipment:     domain.EquipmentBarbell,
		Category:      domain.CategoryStrength,
		Level:         domain.LevelBeginner,
		Source:        domain.SourceFreeExerciseDB,
		MotionSlug:    motionSlug,
	}, time.Now())
	if err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	return ex
}

func TestSetAISupportedHandler(t *testing.T) {
	tests := []struct {
		name       string
		motionSlug string
		eventID    string
		wantErr    error
		wantSaved  bool
		wantLogged bool
	}{
		{name: "admin toggle with motion slug", motionSlug: "bench_press", wantSaved: true},
		{name: "event with motion slug", motionSlug: "bench_press", eventID: "evt-1", wantLogged: true},
		{name: "rejected without motion slug", wantErr: domain.ErrMotionSlugRequired},
		{name: "event rejected without motion slug", eventID: "evt-2", wantErr: domain.ErrMotionSlugRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockExerciseRepo{exercise: newExercise(t, tt.motionSlug)}
			handler := command.NewSetAISupportedHandler(repo, mockClock{now: time.Now()})

			got, err := handler.Handle(context.Background(), &command.SetAISupportedCommand{
				ID:        "ex-1",
				Supported: true,
				EventID:   tt.eventID,
			})

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}
			if (repo.saved != nil) != tt.wantSaved {
				t.Errorf("saved = %v, want %v", repo.saved != nil, tt.wantSaved)
			}
			if (repo.loggedFor != "") != tt.wantLogged {
				t.Errorf("outbox-logged = %q, want logged %v", repo.loggedFor, tt.wantLogged)
			}
			if tt.wantErr == nil && !got.Info().HasAISupported {
				t.Error("expected HasAISupported to be true")
			}
		})
	}
}

func TestSetAISupportedHandler_NotFound(t *testing.T) {
	handler := command.NewSetAISupportedHandler(&mockExerciseRepo{}, mockClock{now: time.Now()})

	_, err := handler.Handle(context.Background(), &command.SetAISupportedCommand{ID: "missing", Supported: true})
	if !errors.Is(err, domain.ErrExerciseNotFound) {
		t.Errorf("got error %v, want %v", err, domain.ErrExerciseNotFound)
	}
}
