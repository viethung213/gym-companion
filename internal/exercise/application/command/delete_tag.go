package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

type DeleteTagCommand struct {
	ID string
}

type DeleteTagHandler struct {
	repo port.Repository
}

func NewDeleteTagHandler(repo port.Repository) *DeleteTagHandler {
	return &DeleteTagHandler{repo: repo}
}

func (h *DeleteTagHandler) Handle(ctx context.Context, cmd *DeleteTagCommand) error {
	if _, err := middleware.RequireAdmin(ctx); err != nil {
		return err
	}

	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return domain.ErrInvalidTag
	}

	if err := h.repo.DeleteTag(ctx, id); err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}

	return nil
}
