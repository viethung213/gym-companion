package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type UpdateBodyPartCommand struct {
	ID   string
	Name string
}

type UpdateBodyPartHandler struct {
	repo port.Repository
}

func NewUpdateBodyPartHandler(repo port.Repository) *UpdateBodyPartHandler {
	return &UpdateBodyPartHandler{repo: repo}
}

func (h *UpdateBodyPartHandler) Handle(ctx context.Context, cmd *UpdateBodyPartCommand) (*port.BodyPart, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return nil, domain.ErrInvalidBodyPart
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidBodyPart
	}

	bp := &port.BodyPart{ID: id, Name: name}
	if err := h.repo.UpdateBodyPart(ctx, bp); err != nil {
		return nil, fmt.Errorf("update body part: %w", err)
	}

	return bp, nil
}
