package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
)

// Catalog provides trusted in-process access to the active exercise catalog.
type Catalog struct {
	repo port.Repository
}

func NewCatalog(repo port.Repository) *Catalog {
	return &Catalog{repo: repo}
}

func (c *Catalog) Search(
	ctx context.Context,
	filters *port.SearchFilters,
) ([]*domain.Exercise, error) {
	exercises, err := c.repo.SearchActive(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("search active exercise catalog: %w", err)
	}

	return exercises, nil
}
