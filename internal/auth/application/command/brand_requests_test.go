//go:build unit

package command

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	"github.com/viethung213/gym-companion/internal/auth/domain/event"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

type mockBrandRequestRepo struct {
	requests map[string]*entity.BrandRequest
}

func newMockBrandRequestRepo() *mockBrandRequestRepo {
	return &mockBrandRequestRepo{
		requests: make(map[string]*entity.BrandRequest),
	}
}

func (m *mockBrandRequestRepo) Create(ctx context.Context, req *entity.BrandRequest) error {
	m.requests[req.ID()] = req
	return nil
}

func (m *mockBrandRequestRepo) Update(ctx context.Context, req *entity.BrandRequest) error {
	if _, ok := m.requests[req.ID()]; !ok {
		return derror.ErrBrandRequestNotFound
	}
	m.requests[req.ID()] = req
	return nil
}

func (m *mockBrandRequestRepo) FindByID(ctx context.Context, id string) (*entity.BrandRequest, error) {
	req, ok := m.requests[id]
	if !ok {
		return nil, derror.ErrBrandRequestNotFound
	}
	return req, nil
}

func (m *mockBrandRequestRepo) FindPendingByUserID(ctx context.Context, userID string) (*entity.BrandRequest, error) {
	for _, req := range m.requests {
		if req.UserID() == userID && req.Status() == entity.BrandRequestStatusPending {
			return req, nil
		}
	}
	return nil, derror.ErrBrandRequestNotFound
}

func (m *mockBrandRequestRepo) List(ctx context.Context, filter repository.ListBrandRequestsFilter) ([]*entity.BrandRequest, int, error) {
	var filtered []*entity.BrandRequest
	for _, req := range m.requests {
		if filter.Status == "" || strings.EqualFold(req.Status(), filter.Status) {
			filtered = append(filtered, req)
		}
	}
	total := len(filtered)
	start := filter.Offset
	if start > total {
		start = total
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}
	return filtered[start:end], total, nil
}

func TestRegisterBrandHandler(t *testing.T) {
	ctx := context.Background()
	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	brandRepo := newMockBrandRequestRepo()
	handler := NewRegisterBrandHandler(brandRepo, userRepo)

	// Create normal user
	user := aggregate.RegisterUser("user-1", "John Doe", aggregate.Identity{}, "")
	_ = userRepo.Create(ctx, user)

	// Create already-brand user
	brandRole, _ := vo.NewRole(vo.RoleBrand)
	brandUser := aggregate.NewUser("user-brand", "Brand Gym", brandRole, aggregate.UserStatusActive, aggregate.Identity{}, time.Now(), time.Now())
	_ = userRepo.Create(ctx, brandUser)

	t.Run("success register brand", func(t *testing.T) {
		res, err := handler.Handle(ctx, RegisterBrandCommand{
			UserID:       "user-1",
			BrandName:    "Alpha Gym",
			Description:  "Best gym",
			ContactPhone: "0901234567",
			Address:      "123 Street",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.BrandName() != "Alpha Gym" {
			t.Errorf("got brand name %s, want Alpha Gym", res.BrandName())
		}
		if res.Status() != entity.BrandRequestStatusPending {
			t.Errorf("got status %s, want pending", res.Status())
		}
	})

	t.Run("duplicate pending request rejected", func(t *testing.T) {
		_, err := handler.Handle(ctx, RegisterBrandCommand{
			UserID:       "user-1",
			BrandName:    "Beta Gym",
			Description:  "Another gym",
			ContactPhone: "0901234567",
			Address:      "456 Street",
		})
		if err != derror.ErrBrandRequestAlreadyPending {
			t.Fatalf("got err %v, want %v", err, derror.ErrBrandRequestAlreadyPending)
		}
	})

	t.Run("user already brand rejected", func(t *testing.T) {
		_, err := handler.Handle(ctx, RegisterBrandCommand{
			UserID:       "user-brand",
			BrandName:    "Super Gym",
			Description:  "Gym",
			ContactPhone: "0901234567",
			Address:      "789 Street",
		})
		if err != derror.ErrUserAlreadyBrand {
			t.Fatalf("got err %v, want %v", err, derror.ErrUserAlreadyBrand)
		}
	})

	t.Run("unauthorized if no user id", func(t *testing.T) {
		_, err := handler.Handle(ctx, RegisterBrandCommand{
			UserID: "",
		})
		if err != derror.ErrUnauthorized {
			t.Fatalf("got err %v, want %v", err, derror.ErrUnauthorized)
		}
	})
}

func TestApproveBrandHandler(t *testing.T) {
	ctx := context.Background()
	userRepo := &mockUserRepo{users: make(map[string]*aggregate.User)}
	brandRepo := newMockBrandRequestRepo()
	txManager := &mockTxManager{}
	handler := NewApproveBrandHandler(brandRepo, userRepo, txManager)

	user := aggregate.RegisterUser("user-2", "Jane Doe", aggregate.Identity{}, "")
	_ = userRepo.Create(ctx, user)

	brandReq, _ := entity.NewBrandRequest("req-2", "user-2", "Jane Fitness", "Desc", "0912345678", "Street", time.Now())
	_ = brandRepo.Create(ctx, brandReq)

	t.Run("success approve", func(t *testing.T) {
		err := handler.Handle(ctx, ApproveBrandCommand{
			AdminID:   "admin-1",
			RequestID: "req-2",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify user role changed to brand
		updatedUser, _ := userRepo.FindByID(ctx, "user-2")
		if updatedUser.Role() != vo.RoleBrand {
			t.Errorf("got user role %s, want %s", updatedUser.Role(), vo.RoleBrand)
		}

		// Verify request status is approved
		updatedReq, _ := brandRepo.FindByID(ctx, "req-2")
		if updatedReq.Status() != entity.BrandRequestStatusApproved {
			t.Errorf("got req status %s, want %s", updatedReq.Status(), entity.BrandRequestStatusApproved)
		}
	})

	t.Run("cannot approve non-pending request", func(t *testing.T) {
		err := handler.Handle(ctx, ApproveBrandCommand{
			AdminID:   "admin-1",
			RequestID: "req-2",
		})
		if err != derror.ErrBrandRequestNotPending {
			t.Fatalf("got err %v, want %v", err, derror.ErrBrandRequestNotPending)
		}
	})

	t.Run("success approve with outbox event publishing", func(t *testing.T) {
		outbox := &mockOutboxWriter{}
		handlerWithOutbox := NewApproveBrandHandler(brandRepo, userRepo, txManager, outbox)

		u := aggregate.RegisterUser("user-event", "Brand User", aggregate.Identity{}, "")
		_ = userRepo.Create(ctx, u)

		req, _ := entity.NewBrandRequest("req-event", "user-event", "Event Brand", "Desc", "0912345678", "Street", time.Now())
		_ = brandRepo.Create(ctx, req)

		err := handlerWithOutbox.Handle(ctx, ApproveBrandCommand{
			AdminID:   "admin-1",
			RequestID: "req-event",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(outbox.events) != 1 {
			t.Fatalf("expected 1 outbox event, got %d", len(outbox.events))
		}
		roleEvent, ok := outbox.events[0].(event.UserRoleUpdatedEvent)
		if !ok {
			t.Fatalf("expected UserRoleUpdatedEvent, got %T", outbox.events[0])
		}
		if roleEvent.UserID != "user-event" || roleEvent.NewRole != "brand" {
			t.Errorf("unexpected event content: %+v", roleEvent)
		}
	})
}

type mockOutboxWriter struct {
	events []event.DomainEvent
}

func (m *mockOutboxWriter) Write(ctx context.Context, ev event.DomainEvent) error {
	m.events = append(m.events, ev)
	return nil
}

func TestRejectBrandHandler(t *testing.T) {
	ctx := context.Background()
	brandRepo := newMockBrandRequestRepo()
	handler := NewRejectBrandHandler(brandRepo)

	brandReq, _ := entity.NewBrandRequest("req-3", "user-3", "Gym 3", "Desc", "0912345678", "Street", time.Now())
	_ = brandRepo.Create(ctx, brandReq)

	t.Run("success reject", func(t *testing.T) {
		err := handler.Handle(ctx, RejectBrandCommand{
			AdminID:   "admin-1",
			RequestID: "req-3",
			Reason:    "Incomplete registration details",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		updatedReq, _ := brandRepo.FindByID(ctx, "req-3")
		if updatedReq.Status() != entity.BrandRequestStatusRejected {
			t.Errorf("got status %s, want rejected", updatedReq.Status())
		}
		if updatedReq.RejectionReason() != "Incomplete registration details" {
			t.Errorf("got reason %s, want 'Incomplete registration details'", updatedReq.RejectionReason())
		}
	})

	t.Run("cannot reject non-pending request", func(t *testing.T) {
		err := handler.Handle(ctx, RejectBrandCommand{
			AdminID:   "admin-1",
			RequestID: "req-3",
			Reason:    "Another reason",
		})
		if err != derror.ErrBrandRequestNotPending {
			t.Fatalf("got err %v, want %v", err, derror.ErrBrandRequestNotPending)
		}
	})
}
