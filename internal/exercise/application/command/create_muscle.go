package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type CreateMuscleCommand struct {
	Name       string
	BodyPartID string
}

type CreateMuscleHandler struct {
	repo port.Repository
	ids  port.IDGenerator
}

func NewCreateMuscleHandler(repo port.Repository, ids port.IDGenerator) *CreateMuscleHandler {
	return &CreateMuscleHandler{repo: repo, ids: ids}
}

func (h *CreateMuscleHandler) Handle(ctx context.Context, cmd *CreateMuscleCommand) (*port.Muscle, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidMuscle
	}

	bodyPartID := strings.TrimSpace(cmd.BodyPartID)
	if bodyPartID == "" {
		return nil, domain.ErrInvalidMuscle
	}

	id, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate muscle id: %w", err)
	}

	m := &port.Muscle{ID: id, Name: name, BodyPartID: bodyPartID}
	if err := h.repo.CreateMuscle(ctx, m); err != nil {
		return nil, fmt.Errorf("create muscle: %w", err)
	}

	return m, nil
}
