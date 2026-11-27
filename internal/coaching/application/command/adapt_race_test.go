package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// finishingAgent adapts like adaptingAgent, but the athlete completes the
// first upcoming session while the model is generating.
type finishingAgent struct {
	adaptingAgent
	finish string
}

func (a *finishingAgent) Adapt(ctx context.Context, userID string, d *service.AdaptationDecision) ([]*roadmap.SessionPlanInfo, error) {
	drafts, err := a.adaptingAgent.Adapt(ctx, userID, d)
	if s, ok := a.rm.FindSession(a.finish); ok {
		_ = s.MarkCompleted(100, 0, weekOneEnd().Add(9*time.Hour))
	}
	return drafts, err
}

// Reproduces the review finding: a session completed during the generation
// left a draft matching nothing, and the whole adaptation failed ("agent moved
// sessions"), throwing the generation away. The other sessions still adapt;
// the finished one is left as trained.
func TestReviewWeeks_SessionFinishedDuringGenerationIsLeftAlone(t *testing.T) {
	rm := reviewFixture(t)
	complete(t, rm, 100, 0, "w1-s0", "w1-s1", "w1-s2")
	h, _, _ := newReviewHandler(rm, weekOneEnd().Add(8*time.Hour))
	h.adapt.agent = &finishingAgent{adaptingAgent: adaptingAgent{rm: rm}, finish: "w2-s0"}

	res, err := h.Handle(context.Background(), "rm-r")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.Deferred || res.Adaptation == nil {
		t.Fatalf("adaptation deferred or missing: %+v", res)
	}
	if s, _ := rm.FindSession("w2-s1"); s.Info().Reasoning != "adapted" {
		t.Error("w2-s1 was not adapted")
	}
	if s, _ := rm.FindSession("w2-s0"); s.Status() != roadmap.SessionPlanStatusCompleted || s.Info().Reasoning == "adapted" {
		t.Errorf("w2-s0 = %s %q, want left COMPLETED as trained", s.Status(), s.Info().Reasoning)
	}
}
