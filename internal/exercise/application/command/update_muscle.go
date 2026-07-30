package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type UpdateMuscleCommand struct {
	ID         string
	Name       string
	BodyPartID string
}

type UpdateMuscleHandler struct {
	repo port.Repository
}

func NewUpdateMuscleHandler(repo port.Repository) *UpdateMuscleHandler {
	return &UpdateMuscleHandler{repo: repo}
}

func (h *UpdateMuscleHandler) Handle(ctx context.Context, cmd *UpdateMuscleCommand) (*port.Muscle, error) {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return nil, domain.ErrInvalidMuscle
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidMuscle
	}

	bodyPartID := strings.TrimSpace(cmd.BodyPartID)
	if bodyPartID == "" {
		return nil, domain.ErrInvalidMuscle
	}

	m := &port.Muscle{ID: id, Name: name, BodyPartID: bodyPartID}
	if err := h.repo.UpdateMuscle(ctx, m); err != nil {
		return nil, fmt.Errorf("update muscle: %w", err)
	}

	return m, nil
}
