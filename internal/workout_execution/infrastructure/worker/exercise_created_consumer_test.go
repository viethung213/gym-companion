package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/repository"
	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/worker"
)

type mockMotionRepo struct {
	specs map[string]*aggregate.MotionSpecification
}

var _ repository.MotionSpecificationRepository = (*mockMotionRepo)(nil)

func newMockMotionRepo() *mockMotionRepo {
	return &mockMotionRepo{
		specs: make(map[string]*aggregate.MotionSpecification),
	}
}

func (m *mockMotionRepo) Save(ctx context.Context, spec *aggregate.MotionSpecification) error {
	m.specs[spec.ExerciseID()] = spec
	return nil
}

func (m *mockMotionRepo) FindByExerciseID(ctx context.Context, exerciseID string) (*aggregate.MotionSpecification, error) {
	spec, ok := m.specs[exerciseID]
	if !ok {
		return nil, derror.ErrNotFound
	}
	return spec, nil
}

func (m *mockMotionRepo) Delete(ctx context.Context, exerciseID string) error {
	delete(m.specs, exerciseID)
	return nil
}

func (m *mockMotionRepo) List(ctx context.Context, limit, offset int) ([]*aggregate.MotionSpecification, int, error) {
	var res []*aggregate.MotionSpecification
	for _, s := range m.specs {
		res = append(res, s)
	}
	return res, len(res), nil
}

func TestExerciseCreatedConsumer_HandleMessage(t *testing.T) {
	repo := newMockMotionRepo()
	consumer := worker.NewExerciseCreatedConsumer(repo, nil)

	t.Run("valid CloudEvent creates draft motion spec with is_ready = false", func(t *testing.T) {
		payload := []byte(`{
			"specversion": "1.0",
			"id": "evt-123",
			"source": "services/exercise-service",
			"type": "contracts.supporting.exercise.v1.exerciseCreated",
			"time": "2026-07-30T14:55:00Z",
			"datacontenttype": "application/json",
			"data": {
				"exerciseId": "ex-bench-press-101",
				"exercise": {
					"id": "ex-bench-press-101",
					"name": "Barbell Bench Press"
				}
			}
		}`)

		err := consumer.HandleMessage(context.Background(), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		spec, err := repo.FindByExerciseID(context.Background(), "ex-bench-press-101")
		if err != nil {
			t.Fatalf("want spec to exist, got err: %v", err)
		}
		if spec.ExerciseID() != "ex-bench-press-101" {
			t.Errorf("got ExerciseID = %s, want ex-bench-press-101", spec.ExerciseID())
		}
		if spec.IsReady() {
			t.Error("want new draft spec.IsReady() == false")
		}
	})

	t.Run("ignore unrelated event types gracefully", func(t *testing.T) {
		payload := []byte(`{
			"specversion": "1.0",
			"type": "contracts.supporting.exercise.v1.exerciseArchived",
			"data": {
				"exerciseId": "ex-archived-999"
			}
		}`)

		err := consumer.HandleMessage(context.Background(), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, findErr := repo.FindByExerciseID(context.Background(), "ex-archived-999")
		if findErr == nil {
			t.Error("want spec not to be created for archived event")
		}
	})

	t.Run("idempotency - consuming duplicate event does not error or overwrite", func(t *testing.T) {
		now := time.Now().UTC()
		existing := aggregate.RestoreMotionSpecification("ex-dup-1", "http://detector.onnx", "http://skeleton.onnx", "http://rules", "http://dialogue", "front", true, now, now)
		_ = repo.Save(context.Background(), existing)

		payload := []byte(`{
			"specversion": "1.0",
			"type": "contracts.supporting.exercise.v1.exerciseCreated",
			"data": {
				"exerciseId": "ex-dup-1"
			}
		}`)

		err := consumer.HandleMessage(context.Background(), payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		spec, _ := repo.FindByExerciseID(context.Background(), "ex-dup-1")
		if !spec.IsReady() {
			t.Error("existing completed spec should not be overwritten by duplicate ExerciseCreated event")
		}
	})
}
