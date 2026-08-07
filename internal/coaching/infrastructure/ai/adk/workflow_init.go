package adk

import (
	"context"
	"fmt"
	"iter"

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

			res, genErr := c.generateValidatedPlan(nodeCtx, &coachInput, true)
			if genErr != nil {
				return nil, genErr
			}

			// The review loop already ran inside generateValidatedPlan, so by
			// here the plan either cleared the reviewer or exhausted its rounds
			// and carries the objections in res.Issues.
			evaluated := &EvaluationResult{
				IsValid:  !res.Degraded,
				Plan:     *res.Plan,
				Names:    res.Names,
				Degraded: res.Degraded,
				Issues:   res.Issues,
				Review:   res.Review,
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
