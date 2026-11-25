package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// A roadmap is one 4-week block. Its successor is drafted during the final
// week and held as PLANNED; when the block ends, RollOverHandler completes it
// and activates the successor, so the athlete never waits for a new plan and
// generation load is spread over a week instead of piling up at midnight.

// PlanNextBlockHandler drafts the next block of a roadmap in its final week.
type PlanNextBlockHandler struct {
	tx       port.TransactionManager
	repo     port.RoadmapRepository
	rotation port.RoadmapRotation
	planner  port.NextBlockPlanner
	guard    *guardrail.Engine
	profiles port.UserProfileReader
	clock    port.Clock
}

// NewPlanNextBlockHandler wires the handler.
func NewPlanNextBlockHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	rotation port.RoadmapRotation,
	planner port.NextBlockPlanner,
	guard *guardrail.Engine,
	profiles port.UserProfileReader,
	clock port.Clock,
) *PlanNextBlockHandler {
	return &PlanNextBlockHandler{
		tx: tx, repo: repo, rotation: rotation, planner: planner,
		guard: guard, profiles: profiles, clock: clock,
	}
}

// Handle plans the successor of an ACTIVE roadmap in its final week and
// returns the block it planned. It returns nil when it planned nothing: the
// roadmap is not in its final week yet, or a successor is already planned
// (kept as is). A planned block that does not follow the roadmap (left over
// from an earlier one) is replaced.
func (h *PlanNextBlockHandler) Handle(ctx context.Context, roadmapID string) (*roadmap.Roadmap, error) {
	current, err := h.repo.FindByID(ctx, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("load roadmap %s: %w", roadmapID, err)
	}
	if current.Status() != roadmap.StatusActive || h.clock.Now().Before(current.FinalWeekStartsAt()) {
		return nil, nil
	}

	existing, err := h.rotation.FindPlannedByUser(ctx, current.UserID())
	switch {
	case err == nil && existing.Follows(current):
		return nil, nil // already planned; the worker asks again every tick of the final week
	case err == nil:
		slog.InfoContext(ctx, "Discarding a planned roadmap that does not follow the active one",
			slog.String("user_id", current.UserID()), slog.String("planned_roadmap_id", existing.ID()))
		if delErr := h.rotation.DeletePlannedRoadmap(ctx, existing.ID()); delErr != nil {
			return nil, delErr
		}
	case !errors.Is(err, roadmap.ErrRoadmapNotFound):
		return nil, fmt.Errorf("find planned roadmap: %w", err)
	}

	startDate := roadmap.CalendarDate(current.Info().EndDate, time.UTC)
	plannedAt := h.clock.Now() // before the profile is read; see StampPlannedAt
	draft, err := h.planner.PlanRoadmapFrom(ctx, current.UserID(), startDate)
	if err != nil {
		return nil, fmt.Errorf("plan next block: %w", err)
	}
	if stampErr := draft.StampPlannedAt(plannedAt); stampErr != nil {
		return nil, stampErr
	}
	if !draft.Follows(current) {
		return nil, fmt.Errorf("planned block starts %s, want %s",
			draft.Info().StartDate.Format(time.DateOnly), startDate.Format(time.DateOnly))
	}
	if vetErr := vetDraft(ctx, h.guard, h.profiles, draft); vetErr != nil {
		return nil, vetErr
	}
	if changedErr := h.profileUnchangedSince(ctx, current.UserID(), plannedAt); changedErr != nil {
		return nil, changedErr
	}
	if holdErr := draft.HoldUntilStart(); holdErr != nil {
		return nil, holdErr
	}

	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		return h.repo.Save(txCtx, draft)
	})
	if errors.Is(err, roadmap.ErrPlannedRoadmapExists) {
		return nil, nil // another run planned it first
	}
	if err != nil {
		return nil, fmt.Errorf("save planned roadmap: %w", err)
	}
	return draft, nil
}

// errProfileChangedWhilePlanning drops a block drafted from a profile that
// changed during the generation; the next run plans it again.
var errProfileChangedWhilePlanning = errors.New("profile changed while the next block was planned; planning again next run")

// profileUnchangedSince refuses a draft when the athlete edited their profile
// after planning started. The profile-change event cannot catch it: when it
// was handled there was no planned block yet to discard. The profile RPC
// reports updated_at in whole seconds (RFC 3339), so the comparison does too: a
// same-second edit re-plans.
func (h *PlanNextBlockHandler) profileUnchangedSince(ctx context.Context, userID string, plannedAt time.Time) error {
	profile, err := h.profiles.GetProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("re-read profile after planning: %w", err)
	}
	if !profile.UpdatedAt.Before(plannedAt.Truncate(time.Second)) {
		return errProfileChangedWhilePlanning
	}
	return nil
}
