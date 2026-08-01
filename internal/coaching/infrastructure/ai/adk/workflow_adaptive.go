package adk

import (
	"context"
	"fmt"
	"iter"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

func (c *CoachingContextAgent) buildAdaptiveCycleAgent(_ context.Context) (agent.Agent, error) {
	node := workflow.NewDynamicNode(
		"adaptive_node",
		func(nodeCtx agent.Context, userID string, _ func(*session.Event) error) (*PlanResult, error) {
			coachInput, fetchErr := workflow.RunNode[CoachInput](nodeCtx, c.fetchNode, userID)
			if fetchErr != nil {
				return nil, fetchErr
			}
			coachInput.Flow = FlowAdaptiveCycle

			var pendErr error
			coachInput, pendErr = workflow.RunNode[CoachInput](nodeCtx, c.pendingNode, coachInput)
			if pendErr != nil {
				return nil, pendErr
			}
			coachInput.AdaptationReason = c.takeReason(nodeCtx.SessionID())

			res, genErr := c.generateValidatedPlan(nodeCtx, &coachInput)
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

	wf, err := workflow.New("adaptive_wf", workflow.Chain(workflow.Start, node))
	if err != nil {
		return nil, fmt.Errorf("new adaptive workflow: %w", err)
	}

	return agent.New(agent.Config{
		Name:        "AdaptiveCycleAgent",
		Description: "Adapts workout sessions based on active injury/fatigue signals.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return wf.Run(ic)
		},
	})
}
