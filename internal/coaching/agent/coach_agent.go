// Package agent groups the CoachAgent contract, its context payload, and
// all agent-side implementation (prompt builder, LLM adapters, tools).
// Coaching (application/command, application/query) depends only on the
// exported facade types from this package — never on its sub-packages.
package agent

import (
	"context"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// FlowType identifies which coaching workflow invoked the agent.
type FlowType string

const (
	FlowInitiate4Week FlowType = "INITIATE_4_WEEK"
	FlowRegenerate    FlowType = "REGENERATE_PENDING"
	FlowAdaptiveCycle FlowType = "ADAPTIVE_CYCLE"
	FlowSignalHandler FlowType = "SIGNAL_HANDLER"
	FlowPostInjury    FlowType = "POST_INJURY_RECOVERY"
	FlowDashboard     FlowType = "DASHBOARD_SUMMARY"
	// FlowSuggestAdHocSession is a read-only flow: user asks the agent to
	// suggest a single ad-hoc workout (outside the roadmap). Nothing is
	// persisted; the frontend decides what to do with the suggestion.
	FlowSuggestAdHocSession FlowType = "SUGGEST_AD_HOC_SESSION"
)

// CoachContext is the payload passed to CoachAgent — a snapshot of everything
// the agent needs to produce a prescription (Profile + history + PR + injuries
// + current roadmap state). Populated by application/contextbuilder.
type CoachContext struct {
	Flow             FlowType
	UserID           string
	Profile          port.Profile
	RecentSessions   []port.WorkoutSession
	InjuryStatus     []port.InjuryStatus
	CurrentRoadmap   *RoadmapSnapshot
	Instructions     string // flow-specific instruction template rendered
	OutputSchemaHint string // JSON schema hint (opaque to Go, used by the LLM adapter)
}

// RoadmapSnapshot is a lightweight read-only view of the current roadmap.
type RoadmapSnapshot struct {
	RoadmapID       string
	Phase           roadmap.Phase
	CurrentWeekNum  int32
	PendingSessions []SessionSnapshot
}

// SessionSnapshot is a read-only view of one SessionPlan.
type SessionSnapshot struct {
	SessionPlanID string
	ScheduledDate string // YYYY-MM-DD
	Phase         roadmap.Phase
	MuscleGroups  []string
}

// Feedback carries reject reasons from Evaluator or Guard when a previous
// draft was rejected. Generator uses this on retry.
type Feedback struct {
	Iteration int
	Issues    []Issue
}

// Issue is one Evaluator/Guard finding.
type Issue struct {
	Code        string
	Description string
	Path        string
	Severity    string // "BLOCKER" or "WARNING"
}

// AdHocHint is what the user (via chat or a simple form) tells the coach when
// asking for a one-off session suggestion.
type AdHocHint struct {
	FreeText           string   // "help me pick chest exercises for 30 minutes"
	MuscleGroups       []string // optional narrowing filter
	AvailableEquipment []string // may differ from profile (e.g. at hotel gym)
	DurationMinutes    int      // 0 = no preference
	IntensityHint      string   // "light" | "normal" | "hard" — optional
}

// SuggestedSession is the read-only output of FlowSuggestAdHocSession. It
// carries a full WorkoutPrescription plus reasoning so the frontend can render
// it and let the user decide (start workout / discard / bookmark).
//
// This value object is intentionally NOT a roadmap.SessionPlan — it never
// enters the aggregate and is not persisted. If the user later commits to
// use it, the frontend hands the exercises off to Workout Execution.
type SuggestedSession struct {
	MuscleGroups []string
	Prescription roadmap.WorkoutPrescription
	Reasoning    string
	// EstimatedRPE is a soft hint the agent used when picking weights.
	EstimatedRPE float32
}

// CoachAgent is the LLM (or deterministic mock) that generates prescriptions.
// It is deliberately stateless w.r.t. downstream steps (Evaluator/Guard) — the
// orchestrator passes Feedback on retry.
type CoachAgent interface {
	// GenerateRoadmap synthesizes a full 4-week draft for a new user.
	GenerateRoadmap(ctx context.Context, cc *CoachContext, fb *Feedback) (*roadmap.Roadmap, error)

	// RegeneratePending rewrites the given PENDING sessions in-place.
	// The returned slice mirrors the ordering of input session IDs.
	RegeneratePending(ctx context.Context, cc *CoachContext, sessionIDs []string, fb *Feedback) ([]*roadmap.SessionPlanInfo, error)

	// Adapt applies a specific AdaptationDecision (Trigger A / Signal B / injury) to
	// the pending portion of the roadmap.
	Adapt(ctx context.Context, cc *CoachContext, decisionReason string, fb *Feedback) ([]*roadmap.SessionPlanInfo, error)

	// SuggestAdHocSession returns a single-session suggestion. Read-only:
	// nothing is persisted, no domain event is emitted. FlowSuggestAdHocSession.
	SuggestAdHocSession(ctx context.Context, cc *CoachContext, hint AdHocHint) (SuggestedSession, error)
}
