package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// Rewrite flows (regenerate, adaptive cycle) get one draft per PENDING session
// they asked the agent to revise. Drafts are matched to sessions by what the
// coach scheduled — calendar day, slot start and kind — never by position: the
// agent may drop a session it could not salvage, and sessions sharing a day
// have no inherent order.

// draftKey identifies a scheduled session.
func draftKey(date time.Time, slotTime string, kind roadmap.SessionKind) string {
	if kind == "" {
		kind = roadmap.SessionKindTraining
	}
	start, _, _ := strings.Cut(slotTime, "-")
	return date.Format(time.DateOnly) + " " + strings.TrimSpace(start) + " " + string(kind)
}

// alignDrafts pairs each pending session with the draft that rewrites it:
// aligned[i] is the draft for pending[i], nil when the agent dropped that
// session. ok is false when any draft matches no pending session, or two
// drafts match the same one: the agent then moved sessions rather than
// rewriting them.
func alignDrafts(pending []*roadmap.SessionPlan, drafts []*roadmap.SessionPlanInfo) (aligned []*roadmap.SessionPlanInfo, ok bool) {
	index := make(map[string]int, len(pending))
	for i, s := range pending {
		info := s.Info()
		index[draftKey(info.ScheduledDate, info.SlotTime, info.Kind)] = i
	}

	aligned = make([]*roadmap.SessionPlanInfo, len(pending))
	for _, d := range drafts {
		if d == nil {
			continue
		}
		i, found := index[draftKey(d.ScheduledDate, d.SlotTime, d.Kind)]
		if !found || aligned[i] != nil {
			return nil, false
		}
		aligned[i] = d
	}
	return aligned, true
}

// rewriteInPlace applies aligned[i] to pending[i]; a nil draft leaves its
// session as it was. It returns how many sessions were rewritten.
func rewriteInPlace(pending []*roadmap.SessionPlan, aligned []*roadmap.SessionPlanInfo, now time.Time) (int, error) {
	applied := 0
	for i, draft := range aligned {
		if draft == nil {
			continue
		}
		s := pending[i]
		if s.Status() != roadmap.SessionPlanStatusPending {
			continue // finalized meanwhile — respect D3
		}
		if err := s.RewritePrescription(draft.Prescription, draft.TargetMuscleGroups, draft.Reasoning, now); err != nil {
			return applied, fmt.Errorf("rewrite session %s: %w", s.ID(), err)
		}
		applied++
	}
	return applied, nil
}
