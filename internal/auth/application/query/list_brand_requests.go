package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// ListBrandRequestsQuery defines parameters for querying brand requests.
type ListBrandRequestsQuery struct {
	Status   string
	Page     int
	PageSize int
}

// ListBrandRequestsResult encapsulates the paginated query results.
type ListBrandRequestsResult struct {
	Items    []*entity.BrandRequest
	Total    int
	Page     int
	PageSize int
}

// ListBrandRequestsHandler processes queries to list brand upgrade requests.
type ListBrandRequestsHandler struct {
	brandRepo repository.BrandRequestRepository
}

// NewListBrandRequestsHandler creates a new instance of ListBrandRequestsHandler.
func NewListBrandRequestsHandler(brandRepo repository.BrandRequestRepository) *ListBrandRequestsHandler {
	return &ListBrandRequestsHandler{
		brandRepo: brandRepo,
	}
}

// Handle executes the query with pagination defaults.
func (h *ListBrandRequestsHandler) Handle(
	ctx context.Context,
	q ListBrandRequestsQuery,
) (*ListBrandRequestsResult, error) {
	page := q.Page
	if page <= 0 {
		page = 1
	}

	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	} else if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	items, total, err := h.brandRepo.List(ctx, repository.ListBrandRequestsFilter{
		Status: q.Status,
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("list brand requests: %w", err)
	}

	return &ListBrandRequestsResult{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
