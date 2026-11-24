// Package command holds application command handlers for the Coaching context.
package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"golang.org/x/sync/singleflight"
)

// InitiateRoadmapCommand is the input to UC-02.1.
type InitiateRoadmapCommand struct {
	UserID string
}

// InitiateRoadmapResult wraps the persisted roadmap snapshot.
type InitiateRoadmapResult struct {
	Roadmap *roadmap.Roadmap
}

// InitiateRoadmapHandler orchestrates: ContextBuilder → CoachAgent → Guard →
// Save (Roadmap + Outbox event) in a single transaction.
type InitiateRoadmapHandler struct {
	tx       port.TransactionManager
	repo     port.RoadmapRepository
	agent    port.CoachAgent
	guard    *guardrail.Engine
	profiles port.UserProfileReader
	outbox   port.OutboxWriter
	clock    port.Clock

	// flight joins concurrent initiations for one user (the onboarding RPC and
	// the ProfileCompleted consumer race) into a single LLM generation.
	flight singleflight.Group
}

// initiateTimeout bounds one roadmap generation, detached from any caller.
const initiateTimeout = 5 * time.Minute

// NewInitiateRoadmapHandler wires the handler.
func NewInitiateRoadmapHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	agent port.CoachAgent,
	guard *guardrail.Engine,
	profiles port.UserProfileReader,
	outbox port.OutboxWriter,
	clock port.Clock,
) *InitiateRoadmapHandler {
	return &InitiateRoadmapHandler{
		tx:       tx,
		repo:     repo,
		agent:    agent,
		guard:    guard,
		profiles: profiles,
		outbox:   outbox,
		clock:    clock,
	}
}

// Handle executes UC-02.1. Concurrent calls for the same user share one
// generation and its result; the generation is detached from the caller, so a
// caller that gives up still leaves the roadmap saved. A roadmap that already
// exists yields roadmap.ErrActiveRoadmapExists.
func (h *InitiateRoadmapHandler) Handle(ctx context.Context, cmd InitiateRoadmapCommand) (*InitiateRoadmapResult, error) {
	if cmd.UserID == "" {
		return nil, errors.New("user_id is required")
	}

	ch := h.flight.DoChan(cmd.UserID, func() (any, error) {
		genCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), initiateTimeout)
		defer cancel()
		return h.initiate(genCtx, cmd)
	})
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for roadmap initiation: %w", ctx.Err())
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		out, _ := res.Val.(*InitiateRoadmapResult)
		return out, nil
	}
}

func (h *InitiateRoadmapHandler) initiate(ctx context.Context, cmd InitiateRoadmapCommand) (*InitiateRoadmapResult, error) {
	// Reject if an ACTIVE roadmap already exists (BR-AC constraint enforced also
	// at DB level via ux_roadmaps_user_active).

	existing, err := h.repo.FindActiveByUser(ctx, cmd.UserID)

	if err != nil && !errors.Is(err, roadmap.ErrRoadmapNotFound) {
		return nil, fmt.Errorf("check active roadmap: %w", err)
	}

	if existing != nil {
		return nil, roadmap.ErrActiveRoadmapExists
	}

	now := h.clock.Now()

	draft, err := h.agent.GenerateRoadmap(ctx, cmd.UserID)

	if err != nil {
		return nil, fmt.Errorf("agent generate: %w", err)
	}

	if valErr := draft.ValidateFullStructure(); valErr != nil {
		return nil, fmt.Errorf("draft structure: %w", valErr)
	}

	if valErr := validateExercisesResolved(draft); valErr != nil {
		return nil, valErr
	}

	injuries, err := activeInjuries(ctx, h.profiles, cmd.UserID)
	if err != nil {
		return nil, err
	}

	// Every session of a fresh draft is new, so all of them are checked.
	if result := h.guard.Check(draft, injuries, time.Time{}); result.Status != guardrail.StatusApproved {
		return nil, fmt.Errorf("guardrail rejected: %+v", result.Violations)
	}

	var out *InitiateRoadmapResult

	err = h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if saveErr := h.repo.Save(txCtx, draft); saveErr != nil {
			return saveErr
		}

		evt := &domainevent.RoadmapInitiated{
			RoadmapID:   draft.ID(),
			UserID:      draft.UserID(),
			InitiatedAt: now,
		}

		if enqErr := h.outbox.Enqueue(txCtx, draft.UserID(), evt); enqErr != nil {
			return enqErr
		}

		out = &InitiateRoadmapResult{Roadmap: draft}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return out, nil
}

// activeInjuries loads the user's unrecovered injuries for the guardrail (BR-AC-09).
func activeInjuries(ctx context.Context, profiles port.UserProfileReader, userID string) ([]service.Injury, error) {
	profile, err := profiles.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load injuries for guardrail: %w", err)
	}
	athlete := profile.Athlete()
	return athlete.Injuries, nil
}

// validateExercisesResolved requires an ID and a catalog-resolved name on every
// exercise. A blank name means catalog validation was skipped upstream.
func validateExercisesResolved(rm *roadmap.Roadmap) error {
	for _, w := range rm.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				p := s.Info().Prescription
				for _, group := range [][]roadmap.PrescribedExercise{
					p.WarmUps, p.MainExercises, p.CoolDowns,
				} {
					for _, ex := range group {
						if ex.ExerciseID == "" {
							return fmt.Errorf("session %s: prescribed exercise has no exercise_id", s.ID())
						}
						if ex.ExerciseName == "" {
							return fmt.Errorf(
								"session %s: exercise %s has no catalog-resolved name; it was not validated against the catalog",
								s.ID(), ex.ExerciseID)
						}
					}
				}
			}
		}
	}
	return nil
}
