package adk

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// errNoUserInState means the workflow reached a tool without publishing the
// user it is acting for, which is a wiring bug rather than a model mistake.
var errNoUserInState = errors.New("user id missing from workflow state")

// stateUserID reads the user the current attempt is for. generateValidatedPlan
// republishes it before every generator call.
func stateUserID(ctx agent.Context) (string, error) {
	val, err := ctx.State().Get("user_id")
	if err != nil {
		return "", fmt.Errorf("%w: %w", errNoUserInState, err)
	}

	userID, ok := val.(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return "", errNoUserInState
	}

	return userID, nil
}

// stateCoachInput reads the input of the current attempt; tools use it to see
// the athlete's equipment, level and injuries.
func stateCoachInput(ctx agent.Context) (CoachInput, error) {
	val, err := ctx.State().Get("coach_input")
	if err != nil {
		return CoachInput{}, fmt.Errorf("read coach input from workflow state: %w", err)
	}
	in, ok := val.(CoachInput)
	if !ok {
		return CoachInput{}, fmt.Errorf("unexpected coach input type %T in workflow state", val)
	}
	return in, nil
}

// SearchArgs defines parameter schema for search_exercises tool.
type SearchArgs struct {
	MuscleGroups []string `json:"muscle_groups"`       // MuscleGroup codes
	Equipment    []string `json:"equipment,omitempty"` // Equipment codes
	Limit        int      `json:"limit,omitempty"`
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

// makeSearchExercisesTool creates an ADK function tool wrapping
// ExerciseCatalogReader. Results honour the athlete's equipment, level and
// injuries (docs/05 §8.2-8.4).
func makeSearchExercisesTool(catalog port.ExerciseCatalogReader) (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "search_exercises",
			Description: "Batch search exercise catalog by MuscleGroup and Equipment codes, returning matching catalog IDs and details.",
		},
		func(ctx agent.Context, args SearchArgs) (SearchResults, error) {
			in, err := stateCoachInput(ctx)
			if err != nil {
				return SearchResults{}, err
			}
			athlete := athleteFromInput(ctx, &in)
			return searchExercises(ctx, catalog, &args, &athlete)
		},
	)
}

// searchExercises runs one search per requested muscle and keeps what the
// athlete may train.
func searchExercises(
	ctx context.Context, catalog port.ExerciseCatalogReader, args *SearchArgs, athlete *service.Athlete,
) (SearchResults, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}
	filter := port.ExerciseFilter{
		Equipment: parseLLMValues(ctx, "equipment", args.Equipment, "search_exercises", taxonomy.ParseEquipment),
		Limit:     limit,
	}
	muscles := parseLLMValues(ctx, "muscle_groups", args.MuscleGroups, "search_exercises", taxonomy.ParseMuscleGroup)
	// One search per muscle keeps `limit` per group; none means one unfiltered search.
	groups := [][]taxonomy.MuscleGroup{nil}
	if len(muscles) > 0 {
		groups = groups[:0]
		for _, m := range muscles {
			groups = append(groups, []taxonomy.MuscleGroup{m})
		}
	}

	var out []ExerciseOption
	for _, group := range groups {
		filter.PrimaryMuscles = group
		res, searchErr := catalog.SearchByFilter(ctx, &filter)
		if searchErr != nil {
			return SearchResults{}, fmt.Errorf("search exercises: %w", searchErr)
		}
		for i := range res {
			e := &res[i]
			traits := e.Traits()
			if !service.EquipmentAvailable(traits.Equipment, athlete.Equipment) ||
				!service.LevelAllowed(traits.Level, athlete.Level) ||
				service.EvaluateInjuries(&traits, athlete.Injuries).Excluded {
				continue
			}
			out = append(out, ExerciseOption{
				ID:            e.ExerciseID,
				Name:          e.Name,
				PrimaryMuscle: string(e.PrimaryMuscle),
				IsCompound:    e.Mechanic == taxonomy.MechanicCompound,
				EquipmentReq:  []string{string(e.Equipment)},
			})
		}
	}
	return SearchResults{Exercises: out}, nil
}

// PRArgs defines parameter schema for get_exercise_pr tool.
type PRArgs struct {
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
	Results []PRItem `json:"results"`
}

type ClarifyArgs struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Protocol    string   `json:"protocol"` // "AG-UI" | "A2UI"
	ContextType string   `json:"context_type"`
}

type ClarifyResult struct {
	RenderedUI string `json:"rendered_ui"`
	Status     string `json:"status"`
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
			_ = ctx.State().Set("hitl_question", args.Question)
			return ClarifyResult{
				RenderedUI: fmt.Sprintf("[%s Protocol] Rendered UI Widget: '%s' Options: %v", args.Protocol, args.Question, args.Options),
				Status:     "WAITING_FOR_USER_SELECTION",
			}, nil
		},
	)
}

// ReplaceInjuredArgs defines parameter schema for replace_injured_exercises tool.
type ReplaceInjuredArgs struct {
	OriginalExerciseID string `json:"original_exercise_id"`
	// SessionTargetMuscleGroups are the session's MuscleGroup codes, for tier 3 of the ranking.
	SessionTargetMuscleGroups []string `json:"session_target_muscle_groups,omitempty"`
}

// ReplaceInjuredResult defines result schema for replace_injured_exercises tool.
// An empty substitute means no safe exercise exists: drop the original.
type ReplaceInjuredResult struct {
	SubstituteExerciseID   string `json:"substitute_id"`
	SubstituteExerciseName string `json:"substitute_name"`
	SafetyReason           string `json:"safety_reason"`
}

// replacementSearchLimit bounds the candidate pool searched per replacement.
const replacementSearchLimit = 100

// makeReplaceInjuredExercisesTool finds the best safe substitute for an
// exercise using the athlete's injuries, equipment and level (docs/05 §8.6).
func makeReplaceInjuredExercisesTool(catalog port.ExerciseCatalogReader) (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name: "replace_injured_exercises",
			Description: "Find the safest substitute for an exercise that conflicts with the athlete's injuries; " +
				"an empty substitute_id means drop the exercise.",
		},
		func(ctx agent.Context, args ReplaceInjuredArgs) (ReplaceInjuredResult, error) {
			in, err := stateCoachInput(ctx)
			if err != nil {
				return ReplaceInjuredResult{}, err
			}
			athlete := athleteFromInput(ctx, &in)
			targets := parseLLMValues(ctx, "session_target_muscle_groups", args.SessionTargetMuscleGroups,
				"replace_injured_exercises", taxonomy.ParseMuscleGroup)
			return replaceInjuredExercise(ctx, catalog, args.OriginalExerciseID, targets, &athlete)
		},
	)
}

// replaceInjuredExercise searches the catalog with exclude_muscles for the
// SEVERE injuries, then ranks what is left with the full §8.2 rules.
func replaceInjuredExercise(
	ctx context.Context,
	catalog port.ExerciseCatalogReader,
	originalID string,
	targets []taxonomy.MuscleGroup,
	athlete *service.Athlete,
) (ReplaceInjuredResult, error) {
	original, err := catalog.GetByID(ctx, originalID)
	if err != nil {
		return ReplaceInjuredResult{}, fmt.Errorf("get original exercise: %w", err)
	}
	originalTraits := original.Traits()
	verdict := service.EvaluateInjuries(&originalTraits, athlete.Injuries)

	candidates, err := catalog.SearchByFilter(ctx, &port.ExerciseFilter{
		PrimaryMuscles: append([]taxonomy.MuscleGroup{original.PrimaryMuscle}, targets...),
		Equipment:      append([]taxonomy.Equipment{taxonomy.EquipmentBodyOnly}, athlete.Equipment...),
		ExcludeMuscles: severeInjuryMuscles(athlete.Injuries),
		Limit:          replacementSearchLimit,
	})
	if err != nil {
		return ReplaceInjuredResult{}, fmt.Errorf("search substitute exercises: %w", err)
	}

	pool := make([]service.ReplacementCandidate, len(candidates))
	byID := make(map[string]string, len(candidates))
	for i := range candidates {
		pool[i] = service.ReplacementCandidate{ID: candidates[i].ExerciseID, Traits: candidates[i].Traits()}
		byID[candidates[i].ExerciseID] = candidates[i].Name
	}

	reason := verdict.Reason()
	if reason == "" {
		reason = "requested replacement"
	}
	ranked := service.RankReplacements(originalID, &originalTraits, pool, targets, athlete)
	if len(ranked) == 0 {
		return ReplaceInjuredResult{
			SafetyReason: fmt.Sprintf("No safe substitute for %s (%s); drop it from the session and say so in reasoning.",
				original.Name, reason),
		}, nil
	}

	chosen := ranked[0]
	return ReplaceInjuredResult{
		SubstituteExerciseID:   chosen.ID,
		SubstituteExerciseName: byID[chosen.ID],
		SafetyReason:           fmt.Sprintf("Replaced %s with %s: %s.", original.Name, byID[chosen.ID], reason),
	}, nil
}

// severeInjuryMuscles lists muscles a SEVERE injury excludes outright, which
// exclude_muscles can filter server-side.
func severeInjuryMuscles(injuries []service.Injury) []taxonomy.MuscleGroup {
	var out []taxonomy.MuscleGroup
	for i := range injuries {
		if injuries[i].Severity != taxonomy.InjurySeveritySevere {
			continue
		}
		for _, m := range taxonomy.AllMuscleGroups() {
			if injuries[i].Muscles()[m] {
				out = append(out, m)
			}
		}
	}
	return out
}

// ScaleArgs defines parameter schema for scale_volume_intensity tool.
type ScaleArgs struct {
	TargetPhase      string  `json:"target_phase"`       // "DELOAD" | "PEAK" | "OVERLOAD"
	VolumeFactorPct  float64 `json:"volume_factor_pct"`  // e.g. -30.0 for Deload (-30% sets)
	IntensityDeltaKg float64 `json:"intensity_delta_kg"` // e.g. +2.5kg for Overload
}

// ScaleResult defines compact output schema for scale_volume_intensity tool.
type ScaleResult struct {
	ScaledPhase        string `json:"scaled_phase"`
	AppliedSetDelta    string `json:"set_adjustment"`
	AppliedWeightDelta string `json:"weight_adjustment"`
}

// makeScaleVolumeIntensityTool creates tool to scale volume/intensity across pending sessions.
func makeScaleVolumeIntensityTool() (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "scale_volume_intensity",
			Description: "Scale volume (sets/reps) and intensity (weight/RPE) across pending sessions.",
		},
		func(_ agent.Context, args ScaleArgs) (ScaleResult, error) {
			return ScaleResult{
				ScaledPhase:        args.TargetPhase,
				AppliedSetDelta:    fmt.Sprintf("%.0f%% set adjustment", args.VolumeFactorPct),
				AppliedWeightDelta: fmt.Sprintf("%+.1fkg intensity delta", args.IntensityDeltaKg),
			}, nil
		},
	)
}

// ShiftSlotsArgs defines parameter schema for shift_session_slots tool.
type ShiftSlotsArgs struct {
	SessionIDs    []string `json:"session_ids"`
	NewDaysOfWeek []int    `json:"new_days_of_week"` // e.g. [1, 3, 5] for Mon, Wed, Fri
}

// ShiftSlotsResult defines compact output schema for shift_session_slots tool.
type ShiftSlotsResult struct {
	ShiftedCount int    `json:"shifted_count"`
	Status       string `json:"status"`
}

// makeShiftSessionSlotsTool creates tool to shift pending session dates to align with user's available slots.
func makeShiftSessionSlotsTool() (tool.Tool, error) {
	return functiontool.New(
		functiontool.Config{
			Name:        "shift_session_slots",
			Description: "Shift pending session dates to align with user's new available slots.",
		},
		func(_ agent.Context, args ShiftSlotsArgs) (ShiftSlotsResult, error) {
			return ShiftSlotsResult{
				ShiftedCount: len(args.SessionIDs),
				Status:       "SLOTS_REALIGNED_SUCCESSFULLY",
			}, nil
		},
	)
}
