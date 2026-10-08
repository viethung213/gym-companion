package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type IngestWorkoutActivityCommand struct {
	SessionID       string
	UserID          string
	Title           string
	Caption         string
	MediaURLs       []string
	DurationSeconds int32
	TotalVolumeKg   float64
	ExerciseCount   int32
	TotalSets       int32
	SharedAt        time.Time
}

//nolint:gocritic // command struct is intentionally value-based
func (c IngestWorkoutActivityCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return derror.ErrUnauthorized
	}
	return nil
}

type IngestWorkoutActivityHandler struct {
	feedItemRepo repository.FeedItemRepository
	txManager    port.TransactionManager
}

func NewIngestWorkoutActivityHandler(
	feedItemRepo repository.FeedItemRepository,
	txManager port.TransactionManager,
) *IngestWorkoutActivityHandler {
	return &IngestWorkoutActivityHandler{
		feedItemRepo: feedItemRepo,
		txManager:    txManager,
	}
}

//nolint:gocritic // command struct is intentionally value-based
func (h *IngestWorkoutActivityHandler) Handle(ctx context.Context, cmd IngestWorkoutActivityCommand) error {
	if err := cmd.Validate(); err != nil {
		return err
	}
	exerciseCount := cmd.ExerciseCount
	if exerciseCount <= 0 && (strings.TrimSpace(cmd.Title) != "" || cmd.TotalSets > 0) {
		exerciseCount = 1
	}
	metrics := vo.NewWorkoutMetrics(
		cmd.SessionID,
		cmd.Title,
		cmd.DurationSeconds,
		cmd.TotalVolumeKg,
		exerciseCount,
		cmd.TotalSets,
	)

	item, err := aggregate.NewWorkoutActivityItem(
		uuid.New().String(),
		cmd.UserID,
		cmd.Caption,
		cmd.MediaURLs,
		metrics,
		vo.VisibilityPublic,
		0,
		0,
		cmd.SharedAt,
		cmd.SharedAt,
	)
	if err != nil {
		return fmt.Errorf("create workout activity feed item: %w", err)
	}

	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.feedItemRepo.Create(txCtx, item); err != nil {
			return fmt.Errorf("persist workout activity feed item: %w", err)
		}
		return nil
	})
}
