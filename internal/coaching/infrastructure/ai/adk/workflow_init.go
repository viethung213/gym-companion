package adk

import (
	"context"
	"fmt"
	"iter"
	"log"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

// buildInitWorkflowAgent creates the INITIATE_4_WEEK workflow agent.
//
// Flow: fetch profile & history → generate + validate 4-week plan → evaluate quality.
// The evaluator result is advisory; catalog validation is the hard gate.
func (c *CoachingContextAgent) buildInitWorkflowAgent(_ context.Context) (agent.Agent, error) {
	node := workflow.NewDynamicNode(
		"init_roadmap_workflow",
		func(nodeCtx agent.Context, userID string, _ func(*session.Event) error) (*EvaluationResult, error) {
			coachInput, fetchErr := workflow.RunNode[CoachInput](nodeCtx, c.fetchNode, userID)
			if fetchErr != nil {
				return nil, fetchErr
			}
			coachInput.Flow = FlowInitiate4Week

			res, genErr := c.generateValidatedPlan(nodeCtx, &coachInput)
			if genErr != nil {
				return nil, genErr
			}

			// Evaluate only the validated plan, never one about to be discarded.
			evaluated := &EvaluationResult{
				IsValid:  true,
				Plan:     *res.Plan,
				Names:    res.Names,
				Degraded: res.Degraded,
				Issues:   res.Issues,
			}

			evaluatedMap, evalErr := workflow.RunNode[map[string]any](nodeCtx, c.evaluatorNode, res.Plan)
			if evalErr != nil {
				log.Printf("coaching: evaluator unavailable for user %s, continuing: %v",
					coachInput.Profile.UserID, evalErr)
			} else if evaluatedMap != nil {
				if v, ok := evaluatedMap["is_valid"].(bool); ok {
					evaluated.IsValid = v
				}
				if rawIssues, ok := evaluatedMap["issues"].([]any); ok {
					for _, item := range rawIssues {
						if s, ok := item.(string); ok {
							evaluated.Issues = append(evaluated.Issues, s)
						}
					}
				}
				if !evaluated.IsValid {
					evaluated.Degraded = true
				}
			}

			if setErr := nodeCtx.State().Set("evaluation_result", evaluated); setErr != nil {
				return nil, fmt.Errorf("record evaluation result: %w", setErr)
			}
			c.putResult(nodeCtx.SessionID(), &PlanResult{
				Plan:     res.Plan,
				Names:    res.Names,
				Degraded: evaluated.Degraded,
				Issues:   evaluated.Issues,
			})

			return evaluated, nil
		},
		workflow.NodeConfig{},
	)

	wf, err := workflow.New("init_roadmap_wf", workflow.Chain(workflow.Start, node))
	if err != nil {
		return nil, fmt.Errorf("new init workflow: %w", err)
	}

	return agent.New(agent.Config{
		Name:        "InitRoadmapWorkflowAgent",
		Description: "Orchestrates initial 4-week roadmap generation and quality evaluation.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return wf.Run(ic)
		},
	})
}
