package adk

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"
)

// validateInputSafety blocks generation when every preferred muscle group is
// covered by a SEVERE injury.
func validateInputSafety(ctx agent.Context, _ *model.LLMRequest) (*model.LLMResponse, error) {
	val, err := ctx.State().Get("coach_input")
	if err != nil {
		return nil, nil // State missing, continue execution
	}
	coachInput, ok := val.(CoachInput)
	if !ok || len(coachInput.Profile.PreferredMuscleGroups) == 0 {
		return nil, nil
	}

	severe := severelyInjuredMuscles(ctx, coachInput.Profile.ActiveInjuries)
	for _, muscle := range coachInput.Profile.PreferredMuscleGroups {
		if !severe[taxonomy.MuscleGroup(muscle)] {
			return nil, nil
		}
	}

	return &model.LLMResponse{
		Content: &genai.Content{
			Role: "model",
			Parts: []*genai.Part{
				{Text: "Input Blocked: All preferred muscle groups have SEVERE injuries. Cannot generate program."},
			},
		},
	}, nil
}

// validateToolExecution intercepts search_exercises to block muscle groups
// covered by a SEVERE injury.
func validateToolExecution(ctx agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	slog.InfoContext(ctx, "[tool_trace] executing tool",
		slog.String("tool", t.Name()),
		slog.Any("args", args))
	if t.Name() != "search_exercises" {
		return nil, nil
	}

	val, err := ctx.State().Get("coach_input")
	if err != nil {
		return nil, nil
	}
	coachInput, ok := val.(CoachInput)
	if !ok {
		return nil, nil
	}

	severe := severelyInjuredMuscles(ctx, coachInput.Profile.ActiveInjuries)
	requested, ok := args["muscle_groups"].([]any)
	if !ok {
		return nil, nil
	}
	for _, raw := range requested {
		muscle, isString := raw.(string)
		if !isString {
			continue
		}
		code, known := parseLLMValue(ctx, "search_exercises.muscle_groups", muscle, "tool_args", taxonomy.ParseMuscleGroup)
		if known && severe[code] {
			return map[string]any{
				"error": fmt.Sprintf("Tool call blocked: muscle group '%s' is covered by a SEVERE injury. Choose another muscle group.", code),
			}, nil
		}
	}

	return nil, nil
}

// severelyInjuredMuscles returns every muscle covered by a SEVERE injury.
func severelyInjuredMuscles(ctx context.Context, injuries []InjuryStatus) map[taxonomy.MuscleGroup]bool {
	severe := make(map[taxonomy.MuscleGroup]bool)
	for _, inj := range domainInjuries(ctx, injuries) {
		if inj.Severity != taxonomy.InjurySeveritySevere {
			continue
		}
		for muscle := range inj.Muscles() {
			severe[muscle] = true
		}
	}
	return severe
}
