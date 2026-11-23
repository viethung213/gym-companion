package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

type adHocIntentResult struct {
	MuscleGroups    []string `json:"muscle_groups"`
	Equipment       []string `json:"equipment,omitempty"`
	AvoidMuscles    []string `json:"avoid_muscles,omitempty"`
	DurationMinutes int      `json:"duration_minutes,omitempty"`
	IntensityNote   string   `json:"intensity_note,omitempty"`
}

func (c *CoachingContextAgent) catalogCanonicalLists() (muscles, equipments []string) {
	c.metaMu.RLock()
	defer c.metaMu.RUnlock()

	seenM := make(map[string]bool, len(c.muscleNames))
	for _, n := range c.muscleNames {
		n = strings.TrimSpace(n)
		if n != "" && !seenM[n] {
			seenM[n] = true
			muscles = append(muscles, n)
		}
	}
	sort.Strings(muscles)

	seenE := make(map[string]bool, len(c.equipmentNames))
	for _, n := range c.equipmentNames {
		n = strings.TrimSpace(n)
		if n != "" && !seenE[n] {
			seenE[n] = true
			equipments = append(equipments, n)
		}
	}
	sort.Strings(equipments)
	return muscles, equipments
}

func (c *CoachingContextAgent) decideAdHocIntent(ctx context.Context, freeText string) (*adHocIntentResult, error) {
	if c.llm == nil || strings.TrimSpace(freeText) == "" {
		return nil, nil
	}

	muscles, equipments := c.catalogCanonicalLists()
	muscleJSON, _ := json.Marshal(muscles)
	equipJSON, _ := json.Marshal(equipments)

	prompt := fmt.Sprintf(`You are an expert fitness intent parser. Analyze the user's workout request in ANY language and extract structured training intent.

Canonical muscle/category list (pick ONLY exact strings from this list):
%s

Canonical equipment list (pick ONLY exact strings from this list):
%s

Universal principles:
1. muscle_groups: Select all matching exact strings from the canonical muscle list.
   If the request is general/intensity-only (no specific muscles named), select a balanced 2 to 4 category combination
   — never return only a single auxiliary category like abs alone.
2. equipment: Only include exact strings from the canonical equipment list if the user explicitly
   restricts or mentions equipment. Leave empty [] if unrestricted.
3. avoid_muscles: Include exact strings from the canonical muscle list for any muscles/joints the user says are sore, tired, injured, or should be avoided.
4. duration_minutes: Extract target duration in minutes if mentioned in natural language (e.g., "half an hour" -> 30, "15p" -> 15). Return 0 if not mentioned.
5. intensity_note: Short phrase capturing desired vibe/intensity (e.g., "light recovery", "fast-paced", "heavy strength", "low impact"), or "" if none.

Return strict JSON only matching:
{"muscle_groups": ["..."], "equipment": [], "avoid_muscles": [], "duration_minutes": 0, "intensity_note": ""}

User request: %s`, string(muscleJSON), string(equipJSON), freeText)

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
		return nil, err
	}
	return &parsed, nil
}

func (c *CoachingContextAgent) resolveMuscleIDByName(name string) string {
	c.metaMu.RLock()
	defer c.metaMu.RUnlock()
	target := strings.ToLower(strings.TrimSpace(name))
	for id, n := range c.muscleNames {
		if strings.ToLower(strings.TrimSpace(n)) == target {
			return id
		}
	}
	for id, n := range c.muscleNames {
		if strings.Contains(strings.ToLower(n), target) {
			return id
		}
	}
	return name
}

func (c *CoachingContextAgent) resolveBodyPartIDByName(name string) string {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return ""
	}
	c.metaMu.RLock()
	defer c.metaMu.RUnlock()
	for id, n := range c.bodyPartNames {
		if strings.ToLower(strings.TrimSpace(n)) == target {
			return id
		}
	}
	for id, n := range c.bodyPartNames {
		if strings.Contains(strings.ToLower(n), target) {
			return id
		}
	}
	return ""
}

func (c *CoachingContextAgent) resolveEquipmentIDByName(name string) string {
	c.metaMu.RLock()
	defer c.metaMu.RUnlock()
	target := strings.ToLower(strings.TrimSpace(name))
	for id, n := range c.equipmentNames {
		if strings.ToLower(strings.TrimSpace(n)) == target {
			return id
		}
	}
	return ""
}

func (c *CoachingContextAgent) prefetchAdHocExercises(
	ctx context.Context,
	muscleGroups []string,
	equipment []string,
	avoidMuscles []string,
	experienceLevel string,
	candidates map[string][][]string,
) int {
	if c.catalog == nil || len(muscleGroups) == 0 || candidates == nil {
		return 0
	}

	avoidSet := make(map[string]bool, len(avoidMuscles))
	for _, m := range avoidMuscles {
		avoidSet[strings.ToLower(strings.TrimSpace(m))] = true
	}

	equipIDs := make([]string, 0, len(equipment))
	for _, eq := range equipment {
		if id := c.resolveEquipmentIDByName(eq); id != "" {
			equipIDs = append(equipIDs, id)
		}
	}

	added := 0
	for _, mg := range muscleGroups {
		mg = strings.TrimSpace(mg)
		if mg == "" || avoidSet[strings.ToLower(mg)] || len(candidates[mg]) >= 4 {
			continue
		}
		muscleID := c.resolveMuscleIDByName(mg)
		bodyPartID := c.resolveBodyPartIDByName(mg)
		res, err := c.catalog.SearchByFilter(ctx, &port.ExerciseFilter{
			TargetMuscleID: muscleID,
			EquipmentIDs:   equipIDs,
			Limit:          24,
		})
		if (err != nil || len(res) == 0) && bodyPartID != "" {
			res, err = c.catalog.SearchByFilter(ctx, &port.ExerciseFilter{
				BodyPartID:   bodyPartID,
				EquipmentIDs: equipIDs,
				Limit:        24,
			})
		}
		if err != nil || len(res) == 0 {
			if len(equipIDs) > 0 {
				res, err = c.catalog.SearchByFilter(ctx, &port.ExerciseFilter{
					TargetMuscleID: muscleID,
					Limit:          24,
				})
				if (err != nil || len(res) == 0) && bodyPartID != "" {
					res, err = c.catalog.SearchByFilter(ctx, &port.ExerciseFilter{
						BodyPartID: bodyPartID,
						Limit:      24,
					})
				}
			}
			if err != nil {
				continue
			}
		}
		for i := range res {
			ex := &res[i]
			if len(candidates[mg]) >= 8 {
				break
			}
			if !isDifficultyAllowed(ex.Difficulty, experienceLevel) {
				continue
			}
			// Never re-inject injured muscles/body parts or recovery-only movements into main candidates.
			exMuscle := strings.ToLower(strings.TrimSpace(c.resolveMuscleName(ctx, ex.MuscleGroup)))
			exBP := strings.ToLower(strings.TrimSpace(c.resolveBodyPartName(ctx, ex.BodyPart)))
			if avoidSet[strings.ToLower(strings.TrimSpace(ex.MuscleGroup))] ||
				avoidSet[exMuscle] ||
				(ex.BodyPart != "" && (avoidSet[strings.ToLower(strings.TrimSpace(ex.BodyPart))] || avoidSet[exBP])) {
				continue
			}
			isRecovery := false
			for _, tid := range ex.TagIDs {
				tn := c.resolveTagName(ctx, tid)
				if strings.EqualFold(tn, tagMobilityWarmUp) || strings.EqualFold(tn, tagStretchingCoolDown) {
					isRecovery = true
					break
				}
			}
			if isRecovery {
				continue
			}
			measType := ex.MeasurementType
			if measType == "" {
				measType = "REPS"
			}
			moveType := "Accessory"
			if strings.EqualFold(mg, "Cardiovascular System") || measType == "DURATION" {
				moveType = "Cardio"
			} else if !ex.IsMachineOrCable {
				moveType = "Compound"
			}
			eqResolved := c.resolveEquipmentName(ctx, ex.Equipment)
			if eqResolved == "" {
				eqResolved = "Body Weight"
			}
			candidates[mg] = append(candidates[mg], []string{
				ex.ExerciseID,
				ex.Name,
				eqResolved,
				ex.Difficulty,
				moveType,
				measType,
			})
			added++
		}
	}
	return added
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
