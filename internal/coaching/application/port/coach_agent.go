package port

import (
	"context"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// FlowType identifies which coaching workflow invoked the agent.
type FlowType string

const (
	FlowInitiate4Week       FlowType = "INITIATE_4_WEEK"
	FlowRegenerate          FlowType = "REGENERATE_PENDING"
	FlowAdaptiveCycle       FlowType = "ADAPTIVE_CYCLE"
	FlowSignalHandler       FlowType = "SIGNAL_HANDLER"
	FlowPostInjury          FlowType = "POST_INJURY_RECOVERY"
	FlowSuggestAdHocSession FlowType = "SUGGEST_AD_HOC_SESSION"
)

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

// AdHocHint is what the user tells the coach when asking for a one-off session.
type AdHocHint struct {
	FreeText           string
	MuscleGroups       []string
	AvailableEquipment []string
	DurationMinutes    int
	IntensityHint      string // "light" | "normal" | "hard"
}

// SuggestedSession is the read-only output of FlowSuggestAdHocSession.
type SuggestedSession struct {
	MuscleGroups []string
	Prescription roadmap.WorkoutPrescription
	Reasoning    string
	EstimatedRPE float32
}

// CoachAgent is the port interface for LLM/ADK orchestration.
type CoachAgent interface {
	// GenerateRoadmap synthesizes a full 4-week draft for a new user.
	GenerateRoadmap(ctx context.Context, userID string) (*roadmap.Roadmap, error)
	// RegeneratePending rewrites the given PENDING sessions in-place.
	RegeneratePending(ctx context.Context, userID string, sessionIDs []string) ([]*roadmap.SessionPlanInfo, error)
	// Adapt applies a specific AdaptationDecision (Trigger A / Signal B / injury) to pending sessions.
	Adapt(ctx context.Context, userID string, decisionReason string) ([]*roadmap.SessionPlanInfo, error)
	// SuggestAdHocSession returns a single-session suggestion.
	SuggestAdHocSession(ctx context.Context, userID string, hint AdHocHint) (SuggestedSession, error)
}
