package adk

import "google.golang.org/genai"

// buildGeneratedPlanSchema mirrors GeneratedPlan for the generator's
// structured output. Gemini then always returns well-formed JSON with only
// these fields: live, the model once dropped a brace and lost three weeks,
// and once invented target_rep_min/target_rep_max, each costing a retry.
// A week carries no phase or RPE window: the backend sets them from its
// number, so the model is not asked for them.
func buildGeneratedPlanSchema() *genai.Schema {
	exercise := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"exercise_id":      {Type: genai.TypeString},
			"target_sets":      {Type: genai.TypeInteger},
			"target_reps":      {Type: genai.TypeInteger},
			"duration_seconds": {Type: genai.TypeInteger},
			"target_weight_kg": {Type: genai.TypeNumber},
			"target_rpe":       {Type: genai.TypeNumber},
			"rest_set_sec":     {Type: genai.TypeInteger},
		},
		PropertyOrdering: []string{
			"exercise_id", "target_sets", "target_reps", "duration_seconds",
			"target_weight_kg", "target_rpe", "rest_set_sec",
		},
		Required: []string{"exercise_id", "target_sets", "target_rpe", "rest_set_sec"},
	}
	exercises := &genai.Schema{Type: genai.TypeArray, Items: exercise}

	session := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"scheduled_date":             {Type: genai.TypeString},
			"slot_time":                  {Type: genai.TypeString},
			"session_kind":               {Type: genai.TypeString, Enum: []string{kindTraining, kindRecovery}},
			"estimated_duration_minutes": {Type: genai.TypeInteger},
			"target_muscle_groups":       {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			"reasoning":                  {Type: genai.TypeString},
			"prescription": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"warm_ups":       exercises,
					"main_exercises": exercises,
					"cool_downs":     exercises,
				},
				PropertyOrdering: []string{"warm_ups", "main_exercises", "cool_downs"},
				Required:         []string{"warm_ups", "main_exercises", "cool_downs"},
			},
		},
		PropertyOrdering: []string{
			"scheduled_date", "slot_time", "session_kind", "estimated_duration_minutes",
			"target_muscle_groups", "reasoning", "prescription",
		},
		Required: []string{"scheduled_date", "slot_time", "session_kind", "prescription"},
	}

	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"weeks": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"week_number": {Type: genai.TypeInteger},
						"sessions":    {Type: genai.TypeArray, Items: session},
					},
					PropertyOrdering: []string{"week_number", "sessions"},
					Required:         []string{"week_number", "sessions"},
				},
			},
		},
		Required: []string{"weeks"},
	}
}
