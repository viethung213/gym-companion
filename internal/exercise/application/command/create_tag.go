package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type CreateTagCommand struct {
	Name string
}

type CreateTagHandler struct {
	repo port.Repository
	ids  port.IDGenerator
}

func NewCreateTagHandler(repo port.Repository, ids port.IDGenerator) *CreateTagHandler {
	return &CreateTagHandler{repo: repo, ids: ids}
}

func (h *CreateTagHandler) Handle(ctx context.Context, cmd *CreateTagCommand) (*port.Tag, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidTag
	}

	id, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate tag id: %w", err)
	}

	t := &port.Tag{ID: id, Name: name}
	if err := h.repo.CreateTag(ctx, t); err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}

	return t, nil
}
