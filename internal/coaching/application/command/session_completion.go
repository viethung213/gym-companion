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
	SessionPlanID string
	// UserID is who the workout service says trained. The plan id comes from the
	// client, so a session owned by someone else is treated as not found.
	UserID              string
	TotalActualSets     int
	TotalPrescribedSets int
	AverageActualRPE    float64
	CompletedAt         string // RFC3339 timestamp for logging
}

// SoloMatchCompletion is the completion of a solo-challenge session. A
// challenge is one effort that was either finished or not, so a finished one
// counts as fully done (SCR 100%); it reports no RPE, so ΔRPE stays
// unmeasured. Solo sessions are not part of the scheduled program and never
// feed the weekly adaptation.
func SoloMatchCompletion(planID, userID, completedAt string) CompleteSessionCommand {
	return CompleteSessionCommand{
		SessionPlanID:       planID,
		UserID:              userID,
		TotalActualSets:     1,
		TotalPrescribedSets: 1,
		CompletedAt:         completedAt,
	}
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

		if !ok || !ownedBy(rm, cmd.UserID) {
			return roadmap.ErrSessionNotFound
		}

		// A redelivered event finds the session already done: nothing is saved
		// or announced again, so a workout is never counted twice.
		if session.Status() == roadmap.SessionPlanStatusCompleted {
			out = completedResult(rm, session)
			return nil
		}

		// Auto-count prescribed sets if not provided.
		totalPrescribedSets := cmd.TotalPrescribedSets
		if totalPrescribedSets <= 0 {
			totalPrescribedSets = countTargetSets(session)
		}

		// Compute SCR / ΔRPE.

		scrPct := float32(h.scr.SCR(cmd.TotalActualSets, totalPrescribedSets))

		now := h.clock.Now()

		// No RPE reported (0) leaves ΔRPE unmeasured: read as 0 - target it
		// would say "far too easy" and skew the week's adaptation.
		var delta float32
		var markErr error
		if cmd.AverageActualRPE > 0 {
			delta = float32(h.scr.DeltaRPE(cmd.AverageActualRPE, float64(sessionTargetRPE(rm, session))))
			markErr = session.MarkCompleted(scrPct, delta, now)
		} else {
			markErr = session.MarkCompletedUnrated(scrPct, now)
		}
		if markErr != nil {
			return fmt.Errorf("mark completed: %w", markErr)
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
			EffortReported:  cmd.AverageActualRPE > 0,
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

// completedResult reports a session completed earlier, from what it stored.
func completedResult(rm *roadmap.Roadmap, session *roadmap.SessionPlan) *CompleteSessionResult {
	info := session.Info()
	res := &CompleteSessionResult{RoadmapID: rm.ID(), SessionPlanID: session.ID()}
	if info.SessionSCR != nil {
		res.SCR = *info.SessionSCR
	}
	if info.SessionDeltaRPE != nil {
		res.DeltaRPE = *info.SessionDeltaRPE
	}
	return res
}

// AbortSessionCommand corresponds to WorkoutSessionAborted event.
type AbortSessionCommand struct {
	SessionPlanID string
	UserID        string // see CompleteSessionCommand.UserID
	Reason        string
}

// AbortSessionHandler marks the session as ABORTED (per issue #173:
// distinguishing ABORTED for mid-workout cancellation / 4h timeout vs SKIPPED).
type AbortSessionHandler struct {
	tx     port.TransactionManager
	repo   port.RoadmapRepository
	outbox port.OutboxWriter
	clock  port.Clock
}

// NewAbortSessionHandler wires the handler.
func NewAbortSessionHandler(
	tx port.TransactionManager,
	repo port.RoadmapRepository,
	outbox port.OutboxWriter,
	clock port.Clock,
) *AbortSessionHandler {
	return &AbortSessionHandler{
		tx:     tx,
		repo:   repo,
		outbox: outbox,
		clock:  clock,
	}
}

// Handle applies the ABORTED transition. Idempotent when already ABORTED.
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

		if !ok || !ownedBy(rm, cmd.UserID) {
			return roadmap.ErrSessionNotFound
		}

		switch session.Status() {
		case roadmap.SessionPlanStatusAborted:
			return nil // redelivered: already applied and announced
		case roadmap.SessionPlanStatusCompleted:
			return nil // the workout was finished after all: the completion wins
		}
		if err := session.MarkAborted(cmd.Reason); err != nil {
			return err
		}

		rm.Touch(h.clock.Now())

		if err := h.repo.Save(txCtx, rm); err != nil {
			return err
		}

		evt := &domainevent.RoadmapAdjusted{
			RoadmapID:  rm.ID(),
			UserID:     rm.UserID(),
			Reason:     "workout_session_aborted:" + session.Info().AbortReason,
			AdjustedAt: h.clock.Now(),
		}

		return h.outbox.Enqueue(txCtx, rm.UserID(), evt)
	})
}

// ownedBy reports whether the roadmap belongs to userID. An empty userID comes
// from an event that predates the field and is accepted as before.
func ownedBy(rm *roadmap.Roadmap, userID string) bool {
	return userID == "" || rm.UserID() == userID
}

// sessionTargetRPE is the intensity the athlete was asked for in this session:
// the mean target RPE of its main exercises. ΔRPE compares the reported RPE
// against it, so a RECOVERY session (RPE ≤ 4) inside a PEAK week (RPE 8-9) is
// not scored as "far too easy". Falls back to the week's target when the
// session prescribes none.
func sessionTargetRPE(rm *roadmap.Roadmap, session *roadmap.SessionPlan) float32 {
	var sum float32
	n := 0
	for _, ex := range session.Info().Prescription.MainExercises {
		if ex.TargetRPE > 0 {
			sum += ex.TargetRPE
			n++
		}
	}
	if n > 0 {
		return sum / float32(n)
	}

	weekID := session.Info().WeekPlanID
	for _, w := range rm.Weeks() {
		if w.ID() == weekID {
			return w.Info().TargetRPE
		}
	}
	return 0
}

// sessionMaxTargetRPE is the highest target RPE of the session's main
// exercises, or its mean target RPE when none carries one.
func sessionMaxTargetRPE(rm *roadmap.Roadmap, session *roadmap.SessionPlan) float32 {
	var top float32
	for _, ex := range session.Info().Prescription.MainExercises {
		top = max(top, ex.TargetRPE)
	}
	if top > 0 {
		return top
	}
	return sessionTargetRPE(rm, session)
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
