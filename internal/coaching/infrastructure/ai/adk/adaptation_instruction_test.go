package adk

import (
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// The model is told the adjustment the engine decided, in its own terms. A
// progression is the backend's to apply, so the model is told to keep the
// numbers rather than to raise them itself.
func TestAdaptationInstruction(t *testing.T) {
	tests := []struct {
		decision service.AdaptationDecision
		want     string
	}{
		{service.AdaptationDecision{Kind: service.AdaptationIncreaseLoad, Reason: "r"}, "target_weight_kg as they are; the backend progresses them"},
		{service.AdaptationDecision{Kind: service.AdaptationTriggerDeload, Reason: "r",
			Params: service.AdaptationParams{VolumePctChange: -0.30, WeightPctChange: -0.10}}, "sets by about 30% and target_weight_kg by about 10%"},
		{service.AdaptationDecision{Kind: service.AdaptationProposeExpressFormat, Reason: "r"}, "about 30 minutes"},
	}
	for _, tt := range tests {
		if got := adaptationInstruction(&tt.decision); !strings.Contains(got, tt.want) {
			t.Errorf("kind %v: %q does not contain %q", tt.decision.Kind, got, tt.want)
		}
	}
}
