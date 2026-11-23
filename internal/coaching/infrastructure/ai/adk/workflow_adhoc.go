package adk

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

func (c *CoachingContextAgent) buildSuggestAdHocAgent(_ context.Context) (agent.Agent, error) {
	node := workflow.NewDynamicNode(
		"suggest_adhoc_node",
		func(nodeCtx agent.Context, userID string, _ func(*session.Event) error) (*PlanResult, error) {
			totalStart := time.Now()
			slog.InfoContext(nodeCtx, "[adhoc_trace] start suggest_adhoc_node", slog.String("user_id", userID))

			fetchStart := time.Now()
			coachInput, fetchErr := workflow.RunNode[CoachInput](nodeCtx, c.fetchNode, userID)
			slog.InfoContext(nodeCtx, "[adhoc_trace] fetch_node completed",
				slog.Int64("fetch_ms", time.Since(fetchStart).Milliseconds()),
				slog.Int("muscle_groups", len(coachInput.AvailableExercises)),
				slog.Bool("has_err", fetchErr != nil))
			if fetchErr != nil {
				return nil, fetchErr
			}
			coachInput.Flow = FlowSuggestAdHoc
			if hint := c.takeHint(nodeCtx.SessionID()); hint != nil {
				intensity := hint.IntensityHint
				if budget := computeAdHocVolumeGuidance(hint.DurationMinutes); budget != "" {
					if strings.TrimSpace(intensity) != "" {
						intensity = intensity + " | " + budget
					} else {
						intensity = budget
					}
				}
				coachInput.AdHocHint = &AdHocHintInput{
					FreeText:           hint.FreeText,
					MuscleGroups:       hint.MuscleGroups,
					AvailableEquipment: hint.AvailableEquipment,
					DurationMinutes:    hint.DurationMinutes,
					IntensityHint:      intensity,
				}
			}

			if coachInput.AdHocHint != nil && strings.TrimSpace(coachInput.AdHocHint.FreeText) != "" {
				intentStart := time.Now()
				intent, intentErr := c.decideAdHocIntent(nodeCtx, coachInput.AdHocHint.FreeText)
				if intentErr == nil && intent != nil {
					slog.InfoContext(nodeCtx, "[adhoc_trace] intent_decision completed",
						slog.Int64("intent_ms", time.Since(intentStart).Milliseconds()),
						slog.Any("decided_muscles", intent.MuscleGroups),
						slog.Any("decided_equipment", intent.Equipment),
						slog.Any("avoid_muscles", intent.AvoidMuscles),
						slog.Int("decided_duration", intent.DurationMinutes),
						slog.String("intensity_note", intent.IntensityNote))

					if len(intent.MuscleGroups) > 0 {
						coachInput.AdHocHint.MuscleGroups = intent.MuscleGroups
					}
					if len(intent.Equipment) > 0 {
						coachInput.AdHocHint.AvailableEquipment = intent.Equipment
					}
					if intent.DurationMinutes >= 10 && intent.DurationMinutes <= 120 {
						coachInput.AdHocHint.DurationMinutes = intent.DurationMinutes
						coachInput.AdHocHint.IntensityHint = computeAdHocVolumeGuidance(intent.DurationMinutes)
					}
					if strings.TrimSpace(intent.IntensityNote) != "" {
						if strings.TrimSpace(coachInput.AdHocHint.IntensityHint) != "" {
							coachInput.AdHocHint.IntensityHint += " | Vibe: " + intent.IntensityNote
						} else {
							coachInput.AdHocHint.IntensityHint = "Vibe: " + intent.IntensityNote
						}
					}
					for _, avoid := range intent.AvoidMuscles {
						coachInput.Profile.ActiveInjuries = append(coachInput.Profile.ActiveInjuries, InjuryStatus{
							MuscleGroup: avoid,
							Severity:    "MILD",
						})
					}

					allAvoid := append([]string(nil), intent.AvoidMuscles...)
					for _, inj := range coachInput.Profile.ActiveInjuries {
						if strings.TrimSpace(inj.MuscleGroup) != "" {
							allAvoid = append(allAvoid, inj.MuscleGroup)
						}
					}

					prefetchStart := time.Now()
					added := c.prefetchAdHocExercises(
						nodeCtx,
						coachInput.AdHocHint.MuscleGroups,
						coachInput.AdHocHint.AvailableEquipment,
						allAvoid,
						coachInput.Profile.ExperienceLevel,
						coachInput.AvailableExercises,
					)
					slog.InfoContext(nodeCtx, "[adhoc_trace] prefetch_adhoc completed",
						slog.Int64("prefetch_ms", time.Since(prefetchStart).Milliseconds()),
						slog.Int("added_exercises", added))
				} else {
					slog.InfoContext(nodeCtx, "[adhoc_trace] intent_decision failed",
						slog.Int64("intent_ms", time.Since(intentStart).Milliseconds()),
						slog.Any("error", intentErr))
				}
			}

			genStart := time.Now()
			res, genErr := c.generateValidatedPlan(nodeCtx, &coachInput, false)
			slog.InfoContext(nodeCtx, "[adhoc_trace] generate_validated_plan completed",
				slog.Int64("gen_ms", time.Since(genStart).Milliseconds()),
				slog.Int64("total_ms", time.Since(totalStart).Milliseconds()),
				slog.Bool("has_err", genErr != nil))
			if genErr != nil {
				return nil, genErr
			}

			if setErr := nodeCtx.State().Set("generated_plan", res.Plan); setErr != nil {
				return nil, fmt.Errorf("record generated plan: %w", setErr)
			}
			c.putResult(nodeCtx.SessionID(), res)

			return res, nil
		},
		workflow.NodeConfig{},
	)

	wf, err := workflow.New("suggest_adhoc_wf", workflow.Chain(workflow.Start, node))
	if err != nil {
		return nil, fmt.Errorf("new suggest adhoc workflow: %w", err)
	}

	return agent.New(agent.Config{
		Name:        "SuggestAdHocAgent",
		Description: "Single session suggestion with AG-UI/A2UI HITL question protocol.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return wf.Run(ic)
		},
	})
}
