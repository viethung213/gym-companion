package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	domainevent "github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// CompleteSessionCommand is the input from WorkoutSessionCompleted event.
type CompleteSessionCommand struct {
	SessionPlanID       string
	TotalActualSets     int
	TotalPrescribedSets int
	AverageActualRPE    float64
	CompletedAt         string // RFC3339 timestamp for logging
}

// CompleteSessionResult echoes state.
type CompleteSessionResult struct {
	RoadmapID     string
	SessionPlanID string
	SCR           float32
	DeltaRPE      float32
}

// CompleteSessionHandler transitions a SessionPlan to COMPLETED and computes
// SCR / ΔRPE per BR-AC-04 (D7: scalar formula).
type CompleteSessionHandler struct {
	tx     port.TransactionManager
	repo   port.RoadmapRepository
	scr    *service.SCRCalculator
	outbox port.OutboxWriter
	clock  port.Clock
}

// NewCompleteSessionHandler wires the handler.
func NewCompleteSessionHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	scr *service.SCRCalculator,
	outbox port.OutboxWriter,
	clock port.Clock,
) *CompleteSessionHandler {
	if scr == nil {
		scr = service.NewSCRCalculator()
	}

	return &CompleteSessionHandler{
		tx:     tx,
		repo:   repo,
		scr:    scr,
		outbox: outbox,
		clock:  clock,
	}
}

// Handle applies the transition. Idempotent when the session is already COMPLETED.
func (h *CompleteSessionHandler) Handle(ctx context.Context, cmd CompleteSessionCommand) (*CompleteSessionResult, error) {
	if cmd.SessionPlanID == "" {
		return nil, errors.New("session_plan_id required")
	}

	var out *CompleteSessionResult

	err := h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm, err := h.repo.FindSessionByID(txCtx, cmd.SessionPlanID)

		if err != nil {
			return fmt.Errorf("find session's roadmap: %w", err)
		}

		session, ok := rm.FindSession(cmd.SessionPlanID)

		if !ok {
			return roadmap.ErrSessionNotFound
		}

		// Auto-count prescribed sets if not provided.
		totalPrescribedSets := cmd.TotalPrescribedSets
		if totalPrescribedSets <= 0 {
			totalPrescribedSets = countTargetSets(session)
		}

		// Compute SCR / ΔRPE.

		scrPct := float32(h.scr.SCR(cmd.TotalActualSets, totalPrescribedSets))

		// Target RPE lives in the WeekPlan; find it.

		targetRPE := findTargetRPEForSession(rm, cmd.SessionPlanID)

		delta := float32(h.scr.DeltaRPE(cmd.AverageActualRPE, float64(targetRPE)))

		now := h.clock.Now()

		sessionInfo := session.Info()
		if sessionInfo.Source == roadmap.SessionPlanSourceAdHoc {
			// Signal B3: ad-hoc session completion detected. Log for downstream processing.
		}

		if err := session.MarkCompleted(scrPct, delta, now); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}

		rm.Touch(now)

		if err := h.repo.Save(txCtx, rm); err != nil {
			return err
		}

		evt := &domainevent.SessionPlanExecuted{
			SessionPlanID:   session.ID(),
			RoadmapID:       rm.ID(),
			UserID:          rm.UserID(),
			ExecutedAt:      now,
			SessionSCR:      scrPct,
			SessionDeltaRPE: delta,
		}

		if err := h.outbox.Enqueue(txCtx, rm.UserID(), evt); err != nil {
			return err
		}

		out = &CompleteSessionResult{
			RoadmapID:     rm.ID(),
			SessionPlanID: session.ID(),
			SCR:           scrPct,
			DeltaRPE:      delta,
		}

		return nil
	})

	return out, err
}

// AbortSessionCommand corresponds to WorkoutSessionAborted event.
type AbortSessionCommand struct {
	SessionPlanID string
	Reason        string
}

// AbortSessionHandler marks the session as SKIPPED (per D3/A7 clarification:
// no separate ABORTED state; aborted sessions map to SKIPPED).
type AbortSessionHandler struct {
	tx     port.TransactionManager
	repo   port.RoadmapRepository
	outbox port.OutboxWriter
	clock  port.Clock
}

// NewAbortSessionHandler wires the handler.
func NewAbortSessionHandler(tx port.TransactionManager, repo port.RoadmapRepository, outbox port.OutboxWriter, clock port.Clock) *AbortSessionHandler {
	return &AbortSessionHandler{
		tx:     tx,
		repo:   repo,
		outbox: outbox,
		clock:  clock,
	}
}

// Handle applies the SKIPPED transition. Idempotent when already SKIPPED.
func (h *AbortSessionHandler) Handle(ctx context.Context, cmd AbortSessionCommand) error {
	if cmd.SessionPlanID == "" {
		return errors.New("session_plan_id required")
	}

	return h.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		rm, err := h.repo.FindSessionByID(txCtx, cmd.SessionPlanID)

		if err != nil {
			return err
		}

		session, ok := rm.FindSession(cmd.SessionPlanID)

		if !ok {
			return roadmap.ErrSessionNotFound
		}

		if err := session.MarkSkipped(); err != nil {
			return err
		}

		rm.Touch(h.clock.Now())

		if err := h.repo.Save(txCtx, rm); err != nil {
			return err
		}

		evt := &domainevent.RoadmapAdjusted{
			RoadmapID:  rm.ID(),
			UserID:     rm.UserID(),
			Reason:     "workout_session_aborted:" + cmd.Reason,
			AdjustedAt: h.clock.Now(),
		}

		return h.outbox.Enqueue(txCtx, rm.UserID(), evt)
	})
}

// findTargetRPEForSession finds the target RPE from the WeekPlan containing
// the given session. Returns 0 if not found.
func findTargetRPEForSession(rm *roadmap.Roadmap, sessionPlanID string) float32 {
	for _, w := range rm.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				if s.ID() == sessionPlanID {
					return w.Info().TargetRPE
				}
			}
		}
	}

	return 0
}

// countTargetSets sums target sets from all exercises in the session prescription.
func countTargetSets(session *roadmap.SessionPlan) int {
	info := session.Info()
	total := 0
	for _, ex := range info.Prescription.WarmUps {
		total += int(ex.TargetSets)
	}
	for _, ex := range info.Prescription.MainExercises {
		total += int(ex.TargetSets)
	}
	for _, ex := range info.Prescription.CoolDowns {
		total += int(ex.TargetSets)
	}
	return total
}
