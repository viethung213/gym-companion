package seeder_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
)

type mockExerciseRegistrar struct {
	mu          sync.Mutex
	calls       []*exercisemsg.CreateExerciseRequest
	bpCalls     []*exercisemsg.CreateBodyPartRequest
	eqCalls     []*exercisemsg.CreateEquipmentRequest
	muscleCalls []*exercisemsg.CreateMuscleRequest
	tagCalls    []*exercisemsg.CreateTagRequest
	adminPass   bool
	errToRet    error
	idSequence  int
}

func (m *mockExerciseRegistrar) nextID(prefix string) string {
	m.idSequence++
	return fmt.Sprintf("%s-%d", prefix, m.idSequence)
}

func (m *mockExerciseRegistrar) CreateBodyPart(ctx context.Context, req *exercisemsg.CreateBodyPartRequest) (*exercisemsg.CreateBodyPartResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := middleware.RequireAdmin(ctx); err == nil {
		m.adminPass = true
	}
	if m.errToRet != nil {
		return nil, m.errToRet
	}
	m.bpCalls = append(m.bpCalls, req)
	return &exercisemsg.CreateBodyPartResponse{
		BodyPart: &exercisemsg.BodyPart{Id: m.nextID("bp"), Name: req.GetName()},
	}, nil
}

func (m *mockExerciseRegistrar) CreateEquipment(ctx context.Context, req *exercisemsg.CreateEquipmentRequest) (*exercisemsg.CreateEquipmentResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errToRet != nil {
		return nil, m.errToRet
	}
	m.eqCalls = append(m.eqCalls, req)
	return &exercisemsg.CreateEquipmentResponse{
		Equipment: &exercisemsg.Equipment{Id: m.nextID("eq"), Name: req.GetName()},
	}, nil
}

func (m *mockExerciseRegistrar) CreateMuscle(ctx context.Context, req *exercisemsg.CreateMuscleRequest) (*exercisemsg.CreateMuscleResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errToRet != nil {
		return nil, m.errToRet
	}
	m.muscleCalls = append(m.muscleCalls, req)
	return &exercisemsg.CreateMuscleResponse{
		Muscle: &exercisemsg.Muscle{Id: m.nextID("muscle"), Name: req.GetName(), BodyPartId: req.GetBodyPartId()},
	}, nil
}

func (m *mockExerciseRegistrar) CreateTag(ctx context.Context, req *exercisemsg.CreateTagRequest) (*exercisemsg.CreateTagResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.errToRet != nil {
		return nil, m.errToRet
	}
	m.tagCalls = append(m.tagCalls, req)
	return &exercisemsg.CreateTagResponse{
		Tag: &exercisemsg.Tag{Id: m.nextID("tag"), Name: req.GetName()},
	}, nil
}

func (m *mockExerciseRegistrar) CreateExercise(ctx context.Context, req *exercisemsg.CreateExerciseRequest) (*exercisemsg.CreateExerciseResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	actor, err := middleware.RequireAdmin(ctx)
	if err == nil && actor.IsAdmin() {
		m.adminPass = true
	}

	if m.errToRet != nil {
		return nil, m.errToRet
	}

	m.calls = append(m.calls, req)
	return &exercisemsg.CreateExerciseResponse{}, nil
}

func TestSeedExercises_NilService(t *testing.T) {
	err := seeder.SeedExercises(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error when exerciseService is nil, got nil")
	}
}

func TestSeedExercises_Success(t *testing.T) {
	mock := &mockExerciseRegistrar{}
	ctx := context.Background()

	err := seeder.SeedExercises(ctx, nil, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mock.adminPass {
		t.Errorf("expected admin authorization in context, but RequireAdmin failed")
	}

	// Verify metadata seeded
	if len(mock.bpCalls) == 0 {
		t.Errorf("expected body parts to be seeded, got 0")
	}
	if len(mock.eqCalls) == 0 {
		t.Errorf("expected equipments to be seeded, got 0")
	}
	if len(mock.muscleCalls) == 0 {
		t.Errorf("expected muscles to be seeded, got 0")
	}
	if len(mock.tagCalls) == 0 {
		t.Errorf("expected tags to be seeded, got 0")
	}

	// Verify exercises seeded
	if len(mock.calls) == 0 {
		t.Fatalf("expected exercise calls, got 0")
	}

	// Verify measurement types, statuses and mapped IDs
	hasReps := false
	hasDuration := false

	for _, req := range mock.calls {
		if req.Status != exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE {
			t.Errorf("expected Status ACTIVE, got %v for %s", req.Status, req.Name)
		}
		if req.MeasurementType == exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS {
			hasReps = true
		}
		if req.MeasurementType == exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION {
			hasDuration = true
		}
		// Confirm IDs are mapped (starting with "bp-", "eq-", "muscle-")
		if req.BodyPartId == "" {
			t.Errorf("empty body_part_id for %s", req.Name)
		}
		if req.EquipmentId == "" {
			t.Errorf("empty equipment_id for %s", req.Name)
		}
		if req.TargetMuscleId == "" {
			t.Errorf("empty target_muscle_id for %s", req.Name)
		}
	}

	if !hasReps {
		t.Errorf("expected some exercises to have REPS measurement type")
	}
	if !hasDuration {
		t.Errorf("expected some exercises to have DURATION measurement type")
	}
}

func TestSeedExercises_ContextCancellation(t *testing.T) {
	mock := &mockExerciseRegistrar{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := seeder.SeedExercises(ctx, nil, mock)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}
