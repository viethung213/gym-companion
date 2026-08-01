package adk

import (
	"fmt"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"
)

// validateInputSafety checks if all preferred muscle groups are severely injured.
func validateInputSafety(ctx agent.Context, _ *model.LLMRequest) (*model.LLMResponse, error) {
	val, err := ctx.State().Get("coach_input")
	if err != nil {
		return nil, nil // State missing, continue execution
	}
	coachInput, ok := val.(CoachInput)
	if !ok {
		return nil, nil
	}

	severeInjuries := make(map[string]bool)
	for _, inj := range coachInput.Profile.ActiveInjuries {
		if inj.Severity == "SEVERE" {
			severeInjuries[inj.MuscleGroup] = true
		}
	}

	allSevere := true
	if len(coachInput.Profile.PreferredMuscleGroups) == 0 {
		allSevere = false
	}
	for _, muscle := range coachInput.Profile.PreferredMuscleGroups {
		if !severeInjuries[muscle] {
			allSevere = false
			break
		}
	}

	if allSevere && len(coachInput.Profile.PreferredMuscleGroups) > 0 {
		return &model.LLMResponse{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Input Blocked: All preferred muscle groups have SEVERE injuries. Cannot generate program."},
				},
			},
		}, nil
	}

	return nil, nil
}

// validateToolExecution intercepts search_exercises to block severely injured muscle queries.
func validateToolExecution(ctx agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
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

	severeInjuries := make(map[string]bool)
	for _, inj := range coachInput.Profile.ActiveInjuries {
		if inj.Severity == "SEVERE" {
			severeInjuries[inj.MuscleGroup] = true
		}
	}

	muscle, _ := args["muscle_group"].(string)
	if severeInjuries[muscle] {
		return map[string]any{
			"error": fmt.Sprintf("Tool call blocked: muscle '%s' has a SEVERE injury status. Choose another muscle group.", muscle),
		}, nil
	}

	return nil, nil
}
