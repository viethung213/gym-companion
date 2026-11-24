package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

const intentSource = "adhoc_intent"

// adHocIntentResult is the raw intent JSON returned by the model.
type adHocIntentResult struct {
	MuscleGroups    []string `json:"muscle_groups"`
	Equipment       []string `json:"equipment,omitempty"`
	AvoidMuscles    []string `json:"avoid_muscles,omitempty"`
	AvoidRegions    []string `json:"avoid_regions,omitempty"`
	DurationMinutes int      `json:"duration_minutes,omitempty"`
	IntensityNote   string   `json:"intensity_note,omitempty"`
}

// adHocIntent is the intent with every value parsed against the vocabulary.
type adHocIntent struct {
	MuscleGroups    []taxonomy.MuscleGroup
	Equipment       []taxonomy.Equipment
	AvoidMuscles    []taxonomy.MuscleGroup
	AvoidRegions    []taxonomy.JointRegion
	DurationMinutes int
	IntensityNote   string
}

// parse drops values outside the vocabulary, logging each (docs/05 §8.5).
func (r *adHocIntentResult) parse(ctx context.Context) adHocIntent {
	return adHocIntent{
		MuscleGroups:    parseLLMValues(ctx, "muscle_groups", r.MuscleGroups, intentSource, taxonomy.ParseMuscleGroup),
		Equipment:       parseLLMValues(ctx, "equipment", r.Equipment, intentSource, taxonomy.ParseEquipment),
		AvoidMuscles:    parseLLMValues(ctx, "avoid_muscles", r.AvoidMuscles, intentSource, taxonomy.ParseMuscleGroup),
		AvoidRegions:    parseLLMValues(ctx, "avoid_regions", r.AvoidRegions, intentSource, taxonomy.ParseJointRegion),
		DurationMinutes: r.DurationMinutes,
		IntensityNote:   r.IntensityNote,
	}
}

// avoidInjuries turns avoided regions and muscles into temporary MODERATE
// injuries for this session only; they are not saved to the profile (§8.5).
func (i *adHocIntent) avoidInjuries() []service.Injury {
	out := make([]service.Injury, 0, len(i.AvoidRegions)+len(i.AvoidMuscles))
	for _, r := range i.AvoidRegions {
		out = append(out, service.Injury{Region: r, Severity: taxonomy.InjurySeverityModerate})
	}
	for _, m := range i.AvoidMuscles {
		out = append(out, service.Injury{Muscle: m, Severity: taxonomy.InjurySeverityModerate})
	}
	return out
}

// intentPrompt lists the canonical vocabulary from the contract enums.
func intentPrompt(freeText string) (string, error) {
	muscles, err := json.Marshal(codeStrings(taxonomy.AllMuscleGroups()))
	if err != nil {
		return "", fmt.Errorf("marshal muscle groups: %w", err)
	}
	equipment, err := json.Marshal(codeStrings(taxonomy.AllEquipment()))
	if err != nil {
		return "", fmt.Errorf("marshal equipment: %w", err)
	}
	regions, err := json.Marshal(codeStrings(taxonomy.AllJointRegions()))
	if err != nil {
		return "", fmt.Errorf("marshal joint regions: %w", err)
	}

	return fmt.Sprintf(`You are an expert fitness intent parser. Analyze the user's workout request in ANY language and extract structured training intent.

Canonical muscle group codes (pick ONLY exact codes from this list):
%s

Canonical equipment codes (pick ONLY exact codes from this list):
%s

Canonical joint region codes (pick ONLY exact codes from this list):
%s

Universal principles:
1. muscle_groups: Select all matching muscle group codes.
   If the request is general/intensity-only (no specific muscles named), select a balanced 2 to 4 muscle group combination
   — never return only a single auxiliary group like ABDOMINALS alone.
2. equipment: Only include equipment codes if the user explicitly restricts or mentions equipment. Leave empty [] if unrestricted.
3. avoid_muscles: Muscle group codes the user says are sore, tired, injured, or should be avoided.
4. avoid_regions: Joint region codes the user says hurt or should be spared (e.g. "my knee hurts" -> ["KNEE"]).
5. duration_minutes: Extract target duration in minutes if mentioned in natural language (e.g., "half an hour" -> 30, "15p" -> 15). Return 0 if not mentioned.
6. intensity_note: Short phrase capturing desired vibe/intensity (e.g., "light recovery", "fast-paced", "heavy strength", "low impact"), or "" if none.

Return strict JSON only matching:
{"muscle_groups": ["..."], "equipment": [], "avoid_muscles": [], "avoid_regions": [], "duration_minutes": 0, "intensity_note": ""}

User request: %s`, muscles, equipment, regions, freeText), nil
}

func (c *CoachingContextAgent) decideAdHocIntent(ctx context.Context, freeText string) (*adHocIntent, error) {
	if c.llm == nil || strings.TrimSpace(freeText) == "" {
		return nil, nil
	}

	prompt, err := intentPrompt(freeText)
	if err != nil {
		return nil, err
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText(prompt, genai.RoleUser),
		},
		Config: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	}

	var raw strings.Builder
	for resp, err := range c.llm.GenerateContent(ctx, req, false) {
		if err != nil {
			return nil, err
		}
		if resp != nil && resp.Content != nil {
			for _, part := range resp.Content.Parts {
				if part != nil && part.Text != "" {
					raw.WriteString(part.Text)
				}
			}
		}
	}

	var parsed adHocIntentResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw.String())), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal ad-hoc intent: %w", err)
	}
	intent := parsed.parse(ctx)
	return &intent, nil
}

func computeAdHocVolumeGuidance(durationMin int) string {
	if durationMin <= 0 {
		return ""
	}
	minAccept := int(float64(durationMin) * 0.75)
	maxAccept := int(float64(durationMin) * 1.20)
	if minAccept < 5 {
		minAccept = 5
	}
	auxMin := 3
	if durationMin >= 25 {
		auxMin = 6
	}
	mainMin := durationMin - auxMin
	if mainMin < 4 {
		mainMin = 4
	}
	targetSets := int(float64(mainMin) / 1.5)
	if targetSets < 4 {
		targetSets = 4
	}
	minEx := targetSets / 4
	if minEx < 2 {
		minEx = 2
	}
	maxEx := (targetSets + 2) / 3
	if maxEx < minEx {
		maxEx = minEx
	}
	return fmt.Sprintf(
		"Strict duration budget: %d min (acceptable %d-%d min). Program %d-%d main exercises "+
			"totaling ~%d main sets plus warm-up & cool-down so actual work+rest time lands inside %d-%d min.",
		durationMin, minAccept, maxAccept, minEx, maxEx, targetSets, minAccept, maxAccept,
	)
}
