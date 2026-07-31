//go:build unit

package command

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/metadata"
)

type mockIDGenerator struct {
	nextID string
	err    error
}

func (m *mockIDGenerator) NewID() (string, error) {
	return m.nextID, m.err
}

type mockRepository struct {
	bp    *port.BodyPart
	eq    *port.Equipment
	m     *port.Muscle
	t     *port.Tag
	err   error
	calls map[string]int
}

func newMockRepository() *mockRepository {
	return &mockRepository{calls: make(map[string]int)}
}

func (m *mockRepository) CreateBodyPart(ctx context.Context, bp *port.BodyPart) error {
	m.calls["CreateBodyPart"]++
	if m.err != nil {
		return m.err
	}
	m.bp = bp
	return nil
}

func (m *mockRepository) GetBodyPart(ctx context.Context, id string) (*port.BodyPart, error) {
	m.calls["GetBodyPart"]++
	if m.err != nil {
		return nil, m.err
	}
	if m.bp == nil {
		return nil, domain.ErrBodyPartNotFound
	}
	return m.bp, nil
}

func (m *mockRepository) ListBodyParts(ctx context.Context, limit, offset int) ([]port.BodyPart, int, error) {
	m.calls["ListBodyParts"]++
	if m.err != nil {
		return nil, 0, m.err
	}
	if m.bp == nil {
		return []port.BodyPart{}, 0, nil
	}
	return []port.BodyPart{*m.bp}, 1, nil
}

func (m *mockRepository) UpdateBodyPart(ctx context.Context, bp *port.BodyPart) error {
	m.calls["UpdateBodyPart"]++
	if m.err != nil {
		return m.err
	}
	m.bp = bp
	return nil
}

func (m *mockRepository) DeleteBodyPart(ctx context.Context, id string) error {
	m.calls["DeleteBodyPart"]++
	if m.err != nil {
		return m.err
	}
	m.bp = nil
	return nil
}

func (m *mockRepository) CreateEquipment(ctx context.Context, eq *port.Equipment) error {
	m.calls["CreateEquipment"]++
	if m.err != nil {
		return m.err
	}
	m.eq = eq
	return nil
}

func (m *mockRepository) GetEquipment(ctx context.Context, id string) (*port.Equipment, error) {
	m.calls["GetEquipment"]++
	if m.err != nil {
		return nil, m.err
	}
	if m.eq == nil {
		return nil, domain.ErrEquipmentNotFound
	}
	return m.eq, nil
}

func (m *mockRepository) ListEquipments(ctx context.Context, limit, offset int) ([]port.Equipment, int, error) {
	m.calls["ListEquipments"]++
	if m.err != nil {
		return nil, 0, m.err
	}
	if m.eq == nil {
		return []port.Equipment{}, 0, nil
	}
	return []port.Equipment{*m.eq}, 1, nil
}

func (m *mockRepository) UpdateEquipment(ctx context.Context, eq *port.Equipment) error {
	m.calls["UpdateEquipment"]++
	if m.err != nil {
		return m.err
	}
	m.eq = eq
	return nil
}

func (m *mockRepository) DeleteEquipment(ctx context.Context, id string) error {
	m.calls["DeleteEquipment"]++
	if m.err != nil {
		return m.err
	}
	m.eq = nil
	return nil
}

func (m *mockRepository) CreateMuscle(ctx context.Context, mu *port.Muscle) error {
	m.calls["CreateMuscle"]++
	if m.err != nil {
		return m.err
	}
	m.m = mu
	return nil
}

func (m *mockRepository) GetMuscle(ctx context.Context, id string) (*port.Muscle, error) {
	m.calls["GetMuscle"]++
	if m.err != nil {
		return nil, m.err
	}
	if m.m == nil {
		return nil, domain.ErrMuscleNotFound
	}
	return m.m, nil
}

func (m *mockRepository) ListMuscles(ctx context.Context, bodyPartID string, limit, offset int) ([]port.Muscle, int, error) {
	m.calls["ListMuscles"]++
	if m.err != nil {
		return nil, 0, m.err
	}
	if m.m == nil {
		return []port.Muscle{}, 0, nil
	}
	return []port.Muscle{*m.m}, 1, nil
}

func (m *mockRepository) UpdateMuscle(ctx context.Context, mu *port.Muscle) error {
	m.calls["UpdateMuscle"]++
	if m.err != nil {
		return m.err
	}
	m.m = mu
	return nil
}

func (m *mockRepository) DeleteMuscle(ctx context.Context, id string) error {
	m.calls["DeleteMuscle"]++
	if m.err != nil {
		return m.err
	}
	m.m = nil
	return nil
}

func (m *mockRepository) CreateTag(ctx context.Context, t *port.Tag) error {
	m.calls["CreateTag"]++
	if m.err != nil {
		return m.err
	}
	m.t = t
	return nil
}

func (m *mockRepository) GetTag(ctx context.Context, id string) (*port.Tag, error) {
	m.calls["GetTag"]++
	if m.err != nil {
		return nil, m.err
	}
	if m.t == nil {
		return nil, domain.ErrTagNotFound
	}
	return m.t, nil
}

func (m *mockRepository) ListTags(ctx context.Context, limit, offset int) ([]port.Tag, int, error) {
	m.calls["ListTags"]++
	if m.err != nil {
		return nil, 0, m.err
	}
	if m.t == nil {
		return []port.Tag{}, 0, nil
	}
	return []port.Tag{*m.t}, 1, nil
}

func (m *mockRepository) UpdateTag(ctx context.Context, t *port.Tag) error {
	m.calls["UpdateTag"]++
	if m.err != nil {
		return m.err
	}
	m.t = t
	return nil
}

func (m *mockRepository) DeleteTag(ctx context.Context, id string) error {
	m.calls["DeleteTag"]++
	if m.err != nil {
		return m.err
	}
	m.t = nil
	return nil
}

// Unused methods for Exercise CRUD
func (m *mockRepository) Save(ctx context.Context, exercise *domain.Exercise, event *domain.Event) error {
	return nil
}

func (m *mockRepository) FindByID(ctx context.Context, id string) (*domain.Exercise, error) {
	return nil, nil
}

func (m *mockRepository) SearchActive(ctx context.Context, filters *port.SearchFilters) ([]*domain.Exercise, error) {
	return nil, nil
}

func (m *mockRepository) GetMetadata(ctx context.Context) (port.Metadata, error) {
	return port.Metadata{}, nil
}

// Test BodyPart Create
func TestCreateBodyPart_Success(t *testing.T) {
	t.Parallel()
	repo := newMockRepository()
	ids := &mockIDGenerator{nextID: "bp-123"}
	handler := NewCreateBodyPartHandler(repo, ids)

	adminCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))

	bp, err := handler.Handle(adminCtx, &CreateBodyPartCommand{Name: "Chest"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if bp.ID != "bp-123" || bp.Name != "Chest" {
		t.Errorf("got %+v, want ID=bp-123 Name=Chest", bp)
	}
}

func TestCreateBodyPart_NoAuthz(t *testing.T) {
	t.Parallel()
	repo := newMockRepository()
	ids := &mockIDGenerator{nextID: "bp-123"}
	handler := NewCreateBodyPartHandler(repo, ids)

	userCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	_, err := handler.Handle(userCtx, &CreateBodyPartCommand{Name: "Chest"})
	if err == nil {
		t.Fatalf("expected PermissionDenied error, got nil")
	}
	if !errors.Is(err, middleware.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestCreateBodyPart_EmptyName(t *testing.T) {
	t.Parallel()
	repo := newMockRepository()
	ids := &mockIDGenerator{nextID: "bp-123"}
	handler := NewCreateBodyPartHandler(repo, ids)

	adminCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))

	_, err := handler.Handle(adminCtx, &CreateBodyPartCommand{Name: "  "})
	if !errors.Is(err, domain.ErrInvalidBodyPart) {
		t.Errorf("expected ErrInvalidBodyPart, got %v", err)
	}
}

func TestUpdateBodyPart_Success(t *testing.T) {
	t.Parallel()
	repo := newMockRepository()
	repo.bp = &port.BodyPart{ID: "bp-123", Name: "Old"}
	handler := NewUpdateBodyPartHandler(repo)

	adminCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))

	bp, err := handler.Handle(adminCtx, &UpdateBodyPartCommand{ID: "bp-123", Name: "Updated"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if bp.Name != "Updated" {
		t.Errorf("got Name=%s, want Updated", bp.Name)
	}
}

func TestDeleteBodyPart_Success(t *testing.T) {
	t.Parallel()
	repo := newMockRepository()
	repo.bp = &port.BodyPart{ID: "bp-123", Name: "Chest"}
	handler := NewDeleteBodyPartHandler(repo)

	adminCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))

	err := handler.Handle(adminCtx, &DeleteBodyPartCommand{ID: "bp-123"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.bp != nil {
		t.Errorf("expected body part to be deleted")
	}
}
