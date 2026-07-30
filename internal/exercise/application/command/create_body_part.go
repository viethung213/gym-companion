package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type CreateBodyPartCommand struct {
	Name string
}

type CreateBodyPartHandler struct {
	repo port.Repository
	ids  port.IDGenerator
}

func NewCreateBodyPartHandler(repo port.Repository, ids port.IDGenerator) *CreateBodyPartHandler {
	return &CreateBodyPartHandler{repo: repo, ids: ids}
}

func (h *CreateBodyPartHandler) Handle(ctx context.Context, cmd *CreateBodyPartCommand) (*port.BodyPart, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidBodyPart
	}

	id, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate body part id: %w", err)
	}

	bp := &port.BodyPart{ID: id, Name: name}
	if err := h.repo.CreateBodyPart(ctx, bp); err != nil {
		return nil, fmt.Errorf("create body part: %w", err)
	}

	return bp, nil
}
