package adk

import (
	"context"
	"fmt"
	"iter"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

// The two rewrite flows are one operation with two triggers: REGENERATE_PENDING
// is asked for, ADAPTIVE_CYCLE is provoked by a fatigue or injury signal. Both
// read the active roadmap, rewrite its PENDING sessions and return
// []*SessionPlanInfo, so they share this builder.
//
// They stay two named agents rather than one. The name is how the ADK launcher
// and the A2A server address an agent, and `flow` is what the generator prompt
// branches on: ADAPTIVE_CYCLE additionally applies `adaptation_reason` and any
// active injury protocol.
func (c *CoachingContextAgent) buildRegeneratePendingAgent(context.Context) (agent.Agent, error) {
	return c.buildRewriteAgent(FlowRegenerate, rewriteNames{
		node:  "regenerate_node",
		wf:    "regenerate_wf",
		agent: "RegeneratePendingAgent",
		desc:  "Rewrites specific pending workout sessions.",
	})
}

func (c *CoachingContextAgent) buildAdaptiveCycleAgent(context.Context) (agent.Agent, error) {
	return c.buildRewriteAgent(FlowAdaptiveCycle, rewriteNames{
		node:  "adaptive_node",
		wf:    "adaptive_wf",
		agent: "AdaptiveCycleAgent",
		desc:  "Adapts workout sessions based on active injury/fatigue signals.",
	})
}

// rewriteNames are the four labels that distinguish one rewrite flow from the
// other. They are names only — no behaviour is switched on them.
type rewriteNames struct {
	node  string
	wf    string
	agent string
	desc  string
}

// buildRewriteAgent builds a Fetch → Pending → Generator → Parse → Validate
// flow that revises the PENDING sessions of the active roadmap.
func (c *CoachingContextAgent) buildRewriteAgent(flow string, names rewriteNames) (agent.Agent, error) {
	node := workflow.NewDynamicNode(
		names.node,
		func(nodeCtx agent.Context, userID string, _ func(*session.Event) error) (*PlanResult, error) {
			coachInput, fetchErr := workflow.RunNode[CoachInput](nodeCtx, c.fetchNode, userID)
			if fetchErr != nil {
				return nil, fetchErr
			}
			coachInput.Flow = flow

			var pendErr error
			coachInput, pendErr = workflow.RunNode[CoachInput](nodeCtx, c.pendingNode, coachInput)
			if pendErr != nil {
				return nil, pendErr
			}
			coachInput.AdaptationReason = c.takeReason(nodeCtx.SessionID())

			res, genErr := c.generateValidatedPlan(nodeCtx, &coachInput, true)
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

	wf, err := workflow.New(names.wf, workflow.Chain(workflow.Start, node))
	if err != nil {
		return nil, fmt.Errorf("new %s workflow: %w", names.wf, err)
	}

	return agent.New(agent.Config{
		Name:        names.agent,
		Description: names.desc,
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return wf.Run(ic)
		},
	})
}
