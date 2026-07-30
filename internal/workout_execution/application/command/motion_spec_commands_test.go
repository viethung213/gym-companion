package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
)

type mockMotionSpecRepo struct {
	specs map[string]*aggregate.MotionSpecification
}

var _ repository.MotionSpecificationRepository = (*mockMotionSpecRepo)(nil)

func newMockMotionSpecRepo() *mockMotionSpecRepo {
	return &mockMotionSpecRepo{
		specs: make(map[string]*aggregate.MotionSpecification),
	}
}

func (m *mockMotionSpecRepo) Save(ctx context.Context, spec *aggregate.MotionSpecification) error {
	m.specs[spec.ExerciseID()] = spec
	return nil
}

func (m *mockMotionSpecRepo) FindByExerciseID(ctx context.Context, exerciseID string) (*aggregate.MotionSpecification, error) {
	spec, ok := m.specs[exerciseID]
	if !ok {
		return nil, derror.ErrNotFound
	}
	return spec, nil
}

func (m *mockMotionSpecRepo) Delete(ctx context.Context, exerciseID string) error {
	if _, ok := m.specs[exerciseID]; !ok {
		return derror.ErrNotFound
	}
	delete(m.specs, exerciseID)
	return nil
}

func (m *mockMotionSpecRepo) List(ctx context.Context, limit, offset int) ([]*aggregate.MotionSpecification, int, error) {
	var res []*aggregate.MotionSpecification
	for _, s := range m.specs {
		res = append(res, s)
	}
	return res, len(res), nil
}

// TestUpdateMotionSpecificationHandler covers the primary update flow (draft -> ready).
func TestUpdateMotionSpecificationHandler(t *testing.T) {
	now := time.Now().UTC()
	repo := newMockMotionSpecRepo()
	handler := command.NewUpdateMotionSpecificationHandler(repo, nil, nil)

	t.Run("Update non-existing spec returns error", func(t *testing.T) {
		cmd := command.UpdateMotionSpecificationCommand{
			ExerciseID:   "non-existent-ex",
			OnnxModelURL: "http://cdn/lunge.onnx",
		}

		_, err := handler.Handle(context.Background(), cmd)
		if err == nil {
			t.Fatal("got nil error, want ErrNotFound")
		}
	})

	t.Run("Update existing spec updates fields", func(t *testing.T) {
		draft := aggregate.NewDraftMotionSpecification("ex-lunge")
		_ = repo.Save(context.Background(), draft)

		cmd := command.UpdateMotionSpecificationCommand{
			ExerciseID:             "ex-lunge",
			OnnxModelURL:           "http://cdn/lunge.onnx",
			LocalRulesURL:          "http://cdn/lunge.json",
			DialogueEngineURL:      "http://cdn/lunge_dialogue.json",
			RecommendedCameraAngle: "front",
		}

		spec, err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spec.OnnxModelURL() != "http://cdn/lunge.onnx" {
			t.Errorf("got ONNX URL = %s, want http://cdn/lunge.onnx", spec.OnnxModelURL())
		}
		if !spec.IsReady() {
			t.Error("want spec.IsReady = true after all URLs are set")
		}
	})

	t.Run("Update existing spec preserves is_ready when both URLs present", func(t *testing.T) {
		// Seed a pre-existing draft via RestoreMotionSpecification
		existing := aggregate.RestoreMotionSpecification(
			"ex-squat", "http://cdn/squat.onnx", "http://cdn/squat.json",
			"http://cdn/squat_dialogue.json", "side", false, now, now,
		)
		_ = repo.Save(context.Background(), existing)

		cmd := command.UpdateMotionSpecificationCommand{
			ExerciseID:             "ex-squat",
			OnnxModelURL:           "http://cdn/squat-v2.onnx",
			LocalRulesURL:          "http://cdn/squat-v2.json",
			DialogueEngineURL:      "http://cdn/squat_dialogue-v2.json",
			RecommendedCameraAngle: "side",
		}
		spec, err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !spec.IsReady() {
			t.Error("want spec.IsReady = true after update with all URLs")
		}
	})
}

func TestDeleteMotionSpecificationHandler(t *testing.T) {
	now := time.Now().UTC()
	repo := newMockMotionSpecRepo()
	draft := aggregate.RestoreMotionSpecification("ex-to-del", "a", "b", "c", "side", true, now, now)

	_ = repo.Save(context.Background(), draft)
	handler := command.NewDeleteMotionSpecificationHandler(repo)

	t.Run("Delete existing spec", func(t *testing.T) {
		err := handler.Handle(context.Background(), command.DeleteMotionSpecificationCommand{ExerciseID: "ex-to-del"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, findErr := repo.FindByExerciseID(context.Background(), "ex-to-del")
		if findErr == nil {
			t.Error("want spec to be deleted")
		}
	})
}
