package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type UpdateTagCommand struct {
	ID   string
	Name string
}

type UpdateTagHandler struct {
	repo port.Repository
}

func NewUpdateTagHandler(repo port.Repository) *UpdateTagHandler {
	return &UpdateTagHandler{repo: repo}
}

func (h *UpdateTagHandler) Handle(ctx context.Context, cmd *UpdateTagCommand) (*port.Tag, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return nil, domain.ErrInvalidTag
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidTag
	}

	t := &port.Tag{ID: id, Name: name}
	if err := h.repo.UpdateTag(ctx, t); err != nil {
		return nil, fmt.Errorf("update tag: %w", err)
	}

	return t, nil
}
