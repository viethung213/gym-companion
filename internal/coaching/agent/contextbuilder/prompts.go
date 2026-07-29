package contextbuilder

import (
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/agent"
)

// PromptRegistry renders the instruction template and returns the desired

// JSON output schema hint for a given flow.

type PromptRegistry interface {
	Render(flow agent.FlowType, cc *agent.CoachContext) (instructions, schemaHint string, err error)
}

// StaticPromptRegistry is a simple, in-memory implementation good enough for

// phase-1. Real LLM adapters can replace this with template files.

type StaticPromptRegistry struct{}

// NewStaticPromptRegistry returns the default in-memory registry.

func NewStaticPromptRegistry() *StaticPromptRegistry { return &StaticPromptRegistry{} }

// Render implements PromptRegistry.

func (r *StaticPromptRegistry) Render(flow agent.FlowType, cc *agent.CoachContext) (instructions, schemaHint string, err error) {

	if cc == nil {

		return "", "", errors.New("nil CoachContext")

	}

	switch flow {

	case agent.FlowInitiate4Week:

		return initiatePrompt(cc), schema4WeekRoadmap, nil

	case agent.FlowRegenerate:

		return regeneratePrompt(cc), schemaSessionList, nil

	case agent.FlowAdaptiveCycle:

		return adaptiveCyclePrompt(cc), schemaSessionList, nil

	case agent.FlowSignalHandler:

		return signalPrompt(cc), schemaSessionList, nil

	case agent.FlowPostInjury:

		return postInjuryPrompt(cc), schemaSessionList, nil

	case agent.FlowDashboard:

		return "", "", nil // Dashboard doesn't invoke agent

	case agent.FlowSuggestAdHocSession:

		return suggestAdHocPrompt(cc), schemaSuggestedSession, nil

	default:

		return "", "", fmt.Errorf("unknown flow: %s", flow)

	}

}

func initiatePrompt(cc *agent.CoachContext) string {

	if cc == nil {

		return ""

	}

	return fmt.Sprintf(`Task: Generate a 4-week training roadmap for user %s.

Profile:

  Primary goal: %s

  Available equipment: %v

  Preferred muscle groups: %v

  Available slots: %v

  Active injuries: %d

Constraints:

  - Max 6 training sessions per week (BR-AC-01).

  - Week 1: ACCUMULATION (target RPE 6-7).

  - Week 2: OVERLOAD (target RPE 7-8).

  - Week 3: PEAK (target RPE 8-9).

  - Week 4: DELOAD (target RPE 5-6, volume -30%%, intensity -10%%).

  - Include warmup (5-10 min) and cooldown (5 min) per session.

  - Weight suggestions must be within ±30%% of history/PR.

Output the roadmap as JSON conforming to the schema.`,

		cc.UserID,

		cc.Profile.PrimaryGoal,

		cc.Profile.AvailableEquipment,

		cc.Profile.PreferredMuscleGroups,

		cc.Profile.AvailableSlots,

		len(cc.InjuryStatus),
	)

}

func regeneratePrompt(cc *agent.CoachContext) string {

	if cc == nil {

		return ""

	}

	return fmt.Sprintf(`Task: Regenerate PENDING sessions for user %s in response to profile/injury changes.

Only touch sessions listed in current_roadmap.pending_sessions. Preserve COMPLETED sessions untouched.

Respect the same guardrails as roadmap initiation.`,

		cc.UserID)

}

func adaptiveCyclePrompt(cc *agent.CoachContext) string {

	if cc == nil {

		return ""

	}

	return fmt.Sprintf(`Task: Apply an adaptive-cycle adjustment (BR-AC-04) for user %s based on last week's metrics.

The orchestrator has already decided which rule applies. Your job is to translate that decision into

concrete SessionPlan prescriptions for the next week's PENDING sessions.`,

		cc.UserID)

}

func signalPrompt(cc *agent.CoachContext) string {

	if cc == nil {

		return ""

	}

	return fmt.Sprintf(`Task: Respond to a behavioral signal (B1-B4) for user %s.

Adjust upcoming PENDING sessions per the signal reason. Explain reasoning in the reasoning field of each session.`,

		cc.UserID)

}

func postInjuryPrompt(cc *agent.CoachContext) string {

	if cc == nil {

		return ""

	}

	return fmt.Sprintf(`Task: Post-injury protective adjustment (BR-AC-09) for user %s.

For the next 3 sessions targeting the recovered muscle group:

  - Cap weight at 50%% of pre-injury PR.

  - Prefer machine/cable/bodyweight exercises over free weights.

  - Target RPE ≤ 7 for AI-verified form ≥ 80%%.`,

		cc.UserID)

}

func suggestAdHocPrompt(cc *agent.CoachContext) string {

	if cc == nil {

		return ""

	}

	return fmt.Sprintf(`Task: Suggest a single ad-hoc workout for user %s.

This is a read-only suggestion — nothing will be persisted. Respect:

  - Injuries: %d active

  - Available equipment: %v

  - User preferences: %v

Output a WorkoutPrescription (warmups + main + cooldowns) and 1-line reasoning.`,

		cc.UserID,

		len(cc.InjuryStatus),

		cc.Profile.AvailableEquipment,

		cc.Profile.PreferredMuscleGroups,
	)

}

// JSON schema hints (opaque strings — the LLM adapter is responsible for

// parsing / enforcing them). Phase-1 mock ignores them.

const (
	schema4WeekRoadmap = `{"type":"object","properties":{"weeks":{"type":"array","minItems":4,"maxItems":4}}}`

	schemaSessionList = `{"type":"array","items":{"type":"object","required":["session_plan_id","prescription"]}}`

	schemaSuggestedSession = `{"type":"object","required":["prescription"],"properties":{"prescription":{"type":"object"},"reasoning":{"type":"string"}}}`
)
