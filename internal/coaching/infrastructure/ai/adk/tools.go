package adk

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/adk/v2/tool/skilltoolset"
	"google.golang.org/adk/v2/tool/skilltoolset/skill"
	"google.golang.org/genai"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// SearchArgs defines parameter schema for search_exercises tool.
type SearchArgs struct {
	MuscleGroup string   `json:"muscle_group"`
	Equipment   []string `json:"equipment"`
	Limit       int      `json:"limit"`
}

// ExerciseOption represents a single mapped exercise catalog candidate.
type ExerciseOption struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	PrimaryMuscle string   `json:"primary_muscle"`
	IsCompound    bool     `json:"is_compound"`
	EquipmentReq  []string `json:"equipment_required"`
}

// SearchResults defines result schema for search_exercises tool.
type SearchResults struct {
	Exercises []ExerciseOption `json:"exercises"`
}

// makeSearchExercisesTool creates an ADK function tool wrapping ExerciseCatalogReader.
func makeSearchExercisesTool(catalog port.ExerciseCatalogReader) (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "search_exercises",
			Description: "Search exercise catalog for a target muscle group and available equipment.",
		},
		func(ctx agent.Context, args SearchArgs) (SearchResults, error) {
			filter := port.ExerciseFilter{
				MuscleGroup: args.MuscleGroup,
				Equipment:   args.Equipment,
				Limit:       args.Limit,
			}
			if filter.Limit <= 0 {
				filter.Limit = 5
			}
			res, err := catalog.SearchByFilter(ctx, filter)
			if err != nil {
				return SearchResults{}, fmt.Errorf("search exercise: %w", err)
			}

			out := make([]ExerciseOption, 0, len(res))
			for _, e := range res {
				var eqReq []string
				if e.Equipment != "" {
					eqReq = []string{e.Equipment}
				}
				out = append(out, ExerciseOption{
					ID:            e.ExerciseID,
					Name:          e.Name,
					PrimaryMuscle: e.MuscleGroup,
					IsCompound:    !e.IsMachineOrCable,
					EquipmentReq:  eqReq,
				})
			}
			return SearchResults{Exercises: out}, nil
		},
	)
}

// PRArgs defines parameter schema for get_exercise_pr tool.
type PRArgs struct {
	ExerciseID string `json:"exercise_id"`
}

// PRResults defines result schema for get_exercise_pr tool.
type PRResults struct {
	ExerciseID          string  `json:"exercise_id"`
	Estimated1RMKg      float64 `json:"estimated_1rm_kg"`
	LastSessionWeightKg float64 `json:"last_session_weight_kg"`
	Trend               string  `json:"trend"` // "improving" | "plateau" | "declining"
}

// makeGetExercisePRTool creates an ADK function tool wrapping WorkoutSessionReader logs.
func makeGetExercisePRTool(sessionReader port.WorkoutSessionReader, userID string) (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "get_exercise_pr",
			Description: "Get user's personal record and recent weight history for weight calculation.",
		},
		func(ctx agent.Context, args PRArgs) (PRResults, error) {
			logs, err := sessionReader.GetSetLogs(ctx, userID, args.ExerciseID, 10)
			if err != nil {
				return PRResults{}, fmt.Errorf("get set logs: %w", err)
			}

			if len(logs) == 0 {
				return PRResults{
					ExerciseID: args.ExerciseID,
				}, nil
			}

			var max1RM float64
			var lastWeight float64
			lastWeight = logs[0].Weight

			for _, log := range logs {
				if log.Reps > 0 {
					oneRM := log.Weight * (1.0 + float64(log.Reps)/30.0)
					if oneRM > max1RM {
						max1RM = oneRM
					}
				}
			}

			return PRResults{
				ExerciseID:          args.ExerciseID,
				Estimated1RMKg:      max1RM,
				LastSessionWeightKg: lastWeight,
				Trend:               "plateau",
			}, nil
		},
	)
}

// validateInputSafety checks if all preferred muscle groups are severely injured.
func validateInputSafety(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
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
					{
						Text: "Input Blocked: All preferred muscle groups have SEVERE injuries. Cannot generate program.",
					},
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

// makeInjuryRecoverySkillToolset loads optional injury recovery protocols as an ADK Skill.
func makeInjuryRecoverySkillToolset(ctx context.Context) (tool.Toolset, error) {
	source := skill.NewFileSystemSource(os.DirFS("internal/coaching/infrastructure/ai/adk/skills"))
	return skilltoolset.New(ctx, skilltoolset.Config{
		Source: source,
	})
}

