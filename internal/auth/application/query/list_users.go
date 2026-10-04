package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// ListUsersQuery defines parameters for querying users.
type ListUsersQuery struct {
	Role     string
	Status   string
	Search   string
	Page     int
	PageSize int
}

// ListUsersResult encapsulates the paginated query results.
type ListUsersResult struct {
	Items    []*aggregate.User
	Total    int
	Page     int
	PageSize int
}

// ListUsersHandler processes queries to list users with filtering and pagination.
type ListUsersHandler struct {
	userRepo repository.UserRepository
}

// NewListUsersHandler creates a new instance of ListUsersHandler.
func NewListUsersHandler(userRepo repository.UserRepository) *ListUsersHandler {
	return &ListUsersHandler{
		userRepo: userRepo,
	}
}

// Handle executes the user listing query.
func (h *ListUsersHandler) Handle(
	ctx context.Context,
	q ListUsersQuery,
) (*ListUsersResult, error) {
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

	items, total, err := h.userRepo.List(ctx, repository.ListUsersFilter{
		Role:   strings.TrimSpace(q.Role),
		Status: strings.TrimSpace(q.Status),
		Search: strings.TrimSpace(q.Search),
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return &ListUsersResult{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
