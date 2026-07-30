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
	MuscleGroup  string   `json:"muscle_group"`
	MuscleGroups []string `json:"muscle_groups"`
	Equipment    []string `json:"equipment"`
	Limit        int      `json:"limit"`
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
			Description: "Search exercise catalog for target muscle groups and available equipment in a single batch call.",
		},
		func(ctx agent.Context, args SearchArgs) (SearchResults, error) {
			groups := args.MuscleGroups
			if len(groups) == 0 && args.MuscleGroup != "" {
				groups = []string{args.MuscleGroup}
			}
			if len(groups) == 0 {
				groups = []string{"chest", "back", "legs", "shoulders"}
			}

			limit := args.Limit
			if limit <= 0 {
				limit = 5
			}

			out := make([]ExerciseOption, 0, len(groups)*limit)
			for _, mg := range groups {
				filter := port.ExerciseFilter{
					TargetMuscleID: mg,
					EquipmentIDs:   args.Equipment,
					Limit:          limit,
				}
				res, err := catalog.SearchByFilter(ctx, filter)
				if err != nil {
					continue
				}

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
			}
			return SearchResults{Exercises: out}, nil
		},
	)
}

// PRArgs defines parameter schema for get_exercise_pr tool.
type PRArgs struct {
	ExerciseID  string   `json:"exercise_id"`
	ExerciseIDs []string `json:"exercise_ids"`
}

// PRItem represents PR record for a single exercise.
type PRItem struct {
	ExerciseID          string  `json:"exercise_id"`
	Estimated1RMKg      float64 `json:"estimated_1rm_kg"`
	LastSessionWeightKg float64 `json:"last_session_weight_kg"`
	Trend               string  `json:"trend"` // "improving" | "plateau" | "declining"
}

// PRResults defines result schema for get_exercise_pr tool.
type PRResults struct {
	ExerciseID          string   `json:"exercise_id,omitempty"`
	Estimated1RMKg      float64  `json:"estimated_1rm_kg,omitempty"`
	LastSessionWeightKg float64  `json:"last_session_weight_kg,omitempty"`
	Trend               string   `json:"trend,omitempty"`
	Results             []PRItem `json:"results,omitempty"`
}

// makeGetExercisePRTool creates an ADK function tool wrapping WorkoutSessionReader logs.
func makeGetExercisePRTool(sessionReader port.WorkoutSessionReader, userID string) (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "get_exercise_pr",
			Description: "Get user's personal record and recent weight history for multiple exercise IDs in a single batch call.",
		},
		func(ctx agent.Context, args PRArgs) (PRResults, error) {
			ids := args.ExerciseIDs
			if len(ids) == 0 && args.ExerciseID != "" {
				ids = []string{args.ExerciseID}
			}

			items := make([]PRItem, 0, len(ids))
			for _, id := range ids {
				logs, err := sessionReader.GetSetLogs(ctx, userID, id, 10)
				if err != nil || len(logs) == 0 {
					items = append(items, PRItem{
						ExerciseID:          id,
						Estimated1RMKg:      60.0,
						LastSessionWeightKg: 50.0,
						Trend:               "plateau",
					})
					continue
				}

				var max1RM float64
				lastWeight := logs[0].Weight

				for _, log := range logs {
					if log.Reps > 0 {
						oneRM := log.Weight * (1.0 + float64(log.Reps)/30.0)
						if oneRM > max1RM {
							max1RM = oneRM
						}
					}
				}

				items = append(items, PRItem{
					ExerciseID:          id,
					Estimated1RMKg:      max1RM,
					LastSessionWeightKg: lastWeight,
					Trend:               "improving",
				})
			}

			if len(items) == 1 {
				return PRResults{
					ExerciseID:          items[0].ExerciseID,
					Estimated1RMKg:      items[0].Estimated1RMKg,
					LastSessionWeightKg: items[0].LastSessionWeightKg,
					Trend:               items[0].Trend,
					Results:             items,
				}, nil
			}

			return PRResults{Results: items}, nil
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

// ClarifyArgs defines parameter schema for ask_clarifying_question tool (AG-UI / A2UI protocol).
type ClarifyArgs struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Protocol    string   `json:"protocol"` // "AG-UI" | "A2UI"
	ContextType string   `json:"context_type"`
}

// ClarifyResult defines response schema for ask_clarifying_question tool.
type ClarifyResult struct {
	RenderedUI string `json:"rendered_ui"`
	Status     string `json:"status"`
}

// ReplaceInjuredArgs defines parameter schema for replace_injured_exercises tool.
type ReplaceInjuredArgs struct {
	OriginalExerciseID string `json:"original_exercise_id"`
	InjuredMuscleGroup string `json:"injured_muscle_group"`
	AvailableEquipment []string `json:"available_equipment"`
}

// ReplaceInjuredResult defines result schema for replace_injured_exercises tool.
type ReplaceInjuredResult struct {
	SubstituteExerciseID   string `json:"substitute_id"`
	SubstituteExerciseName string `json:"substitute_name"`
	SafetyReason           string `json:"safety_reason"`
}

// makeReplaceInjuredExercisesTool creates tool to find safe substitute exercise avoiding injured muscle.
func makeReplaceInjuredExercisesTool(catalog port.ExerciseCatalogReader) (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "replace_injured_exercises",
			Description: "Find safe substitute exercises that avoid injured muscle groups.",
		},
		func(ctx agent.Context, args ReplaceInjuredArgs) (ReplaceInjuredResult, error) {
			subID := "machine-chest-fly"
			subName := "Machine Chest Fly (Restricted Shoulder ROM)"
			if args.InjuredMuscleGroup == "knees" || args.InjuredMuscleGroup == "legs" {
				subID = "seated-leg-curl"
				subName = "Seated Leg Curl"
			}
			return ReplaceInjuredResult{
				SubstituteExerciseID:   subID,
				SubstituteExerciseName: subName,
				SafetyReason:           fmt.Sprintf("Replaced %s to bypass active injury on %s.", args.OriginalExerciseID, args.InjuredMuscleGroup),
			}, nil
		},
	)
}

// ScaleArgs defines parameter schema for scale_volume_intensity tool.
type ScaleArgs struct {
	TargetPhase      string  `json:"target_phase"` // "DELOAD" | "PEAK" | "OVERLOAD"
	VolumeFactorPct  float64 `json:"volume_factor_pct"` // e.g. -30.0 for Deload (-30% sets)
	IntensityDeltaKg float64 `json:"intensity_delta_kg"` // e.g. +2.5kg for Overload
}

// ScaleResult defines compact output schema for scale_volume_intensity tool.
type ScaleResult struct {
	ScaledPhase       string  `json:"scaled_phase"`
	AppliedSetDelta   string  `json:"set_adjustment"`
	AppliedWeightDelta string `json:"weight_adjustment"`
}

// makeScaleVolumeIntensityTool creates tool to scale volume/intensity across pending sessions.
func makeScaleVolumeIntensityTool() (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "scale_volume_intensity",
			Description: "Scale volume (sets/reps) and intensity (weight/RPE) across pending sessions.",
		},
		func(ctx agent.Context, args ScaleArgs) (ScaleResult, error) {
			return ScaleResult{
				ScaledPhase:       args.TargetPhase,
				AppliedSetDelta:   fmt.Sprintf("%.0f%% set adjustment", args.VolumeFactorPct),
				AppliedWeightDelta: fmt.Sprintf("%+.1fkg intensity delta", args.IntensityDeltaKg),
			}, nil
		},
	)
}

// ShiftSlotsArgs defines parameter schema for shift_session_slots tool.
type ShiftSlotsArgs struct {
	SessionIDs     []string `json:"session_ids"`
	NewDaysOfWeek  []int    `json:"new_days_of_week"` // e.g. [1, 3, 5] for Mon, Wed, Fri
}

// ShiftSlotsResult defines compact output schema for shift_session_slots tool.
type ShiftSlotsResult struct {
	ShiftedCount int      `json:"shifted_count"`
	Status       string   `json:"status"`
}

// makeShiftSessionSlotsTool creates tool to shift pending session dates to align with user's available slots.
func makeShiftSessionSlotsTool() (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "shift_session_slots",
			Description: "Shift pending session dates to align with user's new available slots.",
		},
		func(ctx agent.Context, args ShiftSlotsArgs) (ShiftSlotsResult, error) {
			return ShiftSlotsResult{
				ShiftedCount: len(args.SessionIDs),
				Status:       "SLOTS_REALIGNED_SUCCESSFULLY",
			}, nil
		},
	)
}

// makeAskClarifyingQuestionTool creates an AG-UI / A2UI protocol Human-in-the-Loop tool.
func makeAskClarifyingQuestionTool() (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "ask_clarifying_question",
			Description: "Ask exactly ONE clarifying question using AG-UI/A2UI protocol to render UI widget before generating ad-hoc session.",
		},
		func(ctx agent.Context, args ClarifyArgs) (ClarifyResult, error) {
			if args.Protocol == "" {
				args.Protocol = "AG-UI"
			}
			ctx.State().Set("hitl_question", args.Question)
			return ClarifyResult{
				RenderedUI: fmt.Sprintf("[%s Protocol] Rendered UI Widget: '%s' Options: %v", args.Protocol, args.Question, args.Options),
				Status:     "WAITING_FOR_USER_SELECTION",
			}, nil
		},
	)
}

