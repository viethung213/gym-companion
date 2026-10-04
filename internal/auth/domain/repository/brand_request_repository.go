package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
)

// ListBrandRequestsFilter defines query filtering and pagination parameters.
type ListBrandRequestsFilter struct {
	Status string
	Offset int
	Limit  int
}

// BrandRequestRepository defines the persistence port for the BrandRequest entity.
type BrandRequestRepository interface {
	Create(ctx context.Context, req *entity.BrandRequest) error
	Update(ctx context.Context, req *entity.BrandRequest) error
	FindByID(ctx context.Context, id string) (*entity.BrandRequest, error)
	FindPendingByUserID(ctx context.Context, userID string) (*entity.BrandRequest, error)
	List(ctx context.Context, filter ListBrandRequestsFilter) ([]*entity.BrandRequest, int, error)
}
