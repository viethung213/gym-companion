package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type GetCatalogMetadataQuery struct{}

type GetCatalogMetadataHandler struct {
	repo port.Repository
}

func NewGetCatalogMetadataHandler(repo port.Repository) *GetCatalogMetadataHandler {
	return &GetCatalogMetadataHandler{
		repo: repo,
	}
}

func (h *GetCatalogMetadataHandler) Handle(ctx context.Context, _ GetCatalogMetadataQuery) (port.Metadata, error) {
	if _, err := middleware.RequireAuthenticated(ctx); err != nil {
		return port.Metadata{}, err
	}

	metadata, err := h.repo.GetMetadata(ctx)
	if err != nil {
		return port.Metadata{}, fmt.Errorf("get metadata: %w", err)
	}

	return metadata, nil
}
