//go:build unit

package query

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type mockQueryRepository struct {
	bp  *port.BodyPart
	bps []port.BodyPart
	err error
}

func (m *mockQueryRepository) GetBodyPart(ctx context.Context, id string) (*port.BodyPart, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.bp == nil {
		return nil, domain.ErrBodyPartNotFound
	}
	return m.bp, nil
}

func (m *mockQueryRepository) ListBodyParts(ctx context.Context, limit, offset int) ([]port.BodyPart, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.bps, len(m.bps), nil
}

func (m *mockQueryRepository) ListEquipments(ctx context.Context, limit, offset int) ([]port.Equipment, int, error) {
	return nil, 0, nil
}

func (m *mockQueryRepository) ListMuscles(ctx context.Context, bodyPartID string, limit, offset int) ([]port.Muscle, int, error) {
	return nil, 0, nil
}

func (m *mockQueryRepository) ListTags(ctx context.Context, limit, offset int) ([]port.Tag, int, error) {
	return nil, 0, nil
}

// Unused Exercise methods
func (m *mockQueryRepository) Save(ctx context.Context, exercise *domain.Exercise, event *domain.Event) error {
	return nil
}

func (m *mockQueryRepository) FindByID(ctx context.Context, id string) (*domain.Exercise, error) {
	return nil, nil
}

func (m *mockQueryRepository) SearchActive(ctx context.Context, filters *port.SearchFilters) ([]*domain.Exercise, error) {
	return nil, nil
}

func (m *mockQueryRepository) GetMetadata(ctx context.Context) (port.Metadata, error) {
	return port.Metadata{}, nil
}

func (m *mockQueryRepository) CreateBodyPart(ctx context.Context, bp *port.BodyPart) error {
	return nil
}

func (m *mockQueryRepository) UpdateBodyPart(ctx context.Context, bp *port.BodyPart) error {
	return nil
}

func (m *mockQueryRepository) DeleteBodyPart(ctx context.Context, id string) error {
	return nil
}

func (m *mockQueryRepository) CreateEquipment(ctx context.Context, eq *port.Equipment) error {
	return nil
}

func (m *mockQueryRepository) GetEquipment(ctx context.Context, id string) (*port.Equipment, error) {
	return nil, nil
}

func (m *mockQueryRepository) UpdateEquipment(ctx context.Context, eq *port.Equipment) error {
	return nil
}

func (m *mockQueryRepository) DeleteEquipment(ctx context.Context, id string) error {
	return nil
}

func (m *mockQueryRepository) CreateMuscle(ctx context.Context, m *port.Muscle) error {
	return nil
}

func (m *mockQueryRepository) GetMuscle(ctx context.Context, id string) (*port.Muscle, error) {
	return nil, nil
}

func (m *mockQueryRepository) UpdateMuscle(ctx context.Context, m *port.Muscle) error {
	return nil
}

func (m *mockQueryRepository) DeleteMuscle(ctx context.Context, id string) error {
	return nil
}

func (m *mockQueryRepository) CreateTag(ctx context.Context, t *port.Tag) error {
	return nil
}

func (m *mockQueryRepository) GetTag(ctx context.Context, id string) (*port.Tag, error) {
	return nil, nil
}

func (m *mockQueryRepository) UpdateTag(ctx context.Context, t *port.Tag) error {
	return nil
}

func (m *mockQueryRepository) DeleteTag(ctx context.Context, id string) error {
	return nil
}

// Test GetBodyPart
func TestGetBodyPart_Success(t *testing.T) {
	t.Parallel()
	repo := &mockQueryRepository{
		bp: &port.BodyPart{ID: "bp-123", Name: "Chest"},
	}
	handler := NewGetBodyPartHandler(repo)

	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	bp, err := handler.Handle(userCtx, GetBodyPartQuery{ID: "bp-123"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if bp.ID != "bp-123" || bp.Name != "Chest" {
		t.Errorf("got %+v, want ID=bp-123 Name=Chest", bp)
	}
}

func TestGetBodyPart_NotFound(t *testing.T) {
	t.Parallel()
	repo := &mockQueryRepository{bp: nil}
	handler := NewGetBodyPartHandler(repo)

	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	_, err := handler.Handle(userCtx, GetBodyPartQuery{ID: "nonexistent"})
	if !errors.Is(err, domain.ErrBodyPartNotFound) {
		t.Errorf("expected ErrBodyPartNotFound, got %v", err)
	}
}

func TestGetBodyPart_NoAuth(t *testing.T) {
	t.Parallel()
	repo := &mockQueryRepository{bp: &port.BodyPart{ID: "bp-123", Name: "Chest"}}
	handler := NewGetBodyPartHandler(repo)

	unauthCtx := context.Background()

	_, err := handler.Handle(unauthCtx, GetBodyPartQuery{ID: "bp-123"})
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
	if !errors.Is(err, middleware.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestListBodyParts_Success(t *testing.T) {
	t.Parallel()
	repo := &mockQueryRepository{
		bps: []port.BodyPart{
			{ID: "bp-1", Name: "Chest"},
			{ID: "bp-2", Name: "Back"},
		},
	}
	handler := NewListBodyPartsHandler(repo)

	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	bps, total, err := handler.Handle(userCtx, ListBodyPartsQuery{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(bps) != 2 || total != 2 {
		t.Errorf("got len=%d total=%d, want len=2 total=2", len(bps), total)
	}
	if bps[0].Name != "Chest" {
		t.Errorf("got %s, want Chest", bps[0].Name)
	}
}

func TestListBodyParts_Empty(t *testing.T) {
	t.Parallel()
	repo := &mockQueryRepository{bps: []port.BodyPart{}}
	handler := NewListBodyPartsHandler(repo)

	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	bps, total, err := handler.Handle(userCtx, ListBodyPartsQuery{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(bps) != 0 || total != 0 {
		t.Errorf("got len=%d total=%d, want len=0 total=0", len(bps), total)
	}
}

func TestListBodyParts_DefaultLimit(t *testing.T) {
	t.Parallel()
	repo := &mockQueryRepository{bps: []port.BodyPart{}}
	handler := NewListBodyPartsHandler(repo)

	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	_, _, err := handler.Handle(userCtx, ListBodyPartsQuery{Limit: 0, Offset: 0})
	if err != nil {
		t.Fatalf("expected no error with default limit, got %v", err)
	}
}
