package adk

import (
	"fmt"
	"math"

	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// adaptationInstruction turns the adaptive engine's decision into what the
// coach must change in the remaining sessions. The rule that fired stays visible as the reason; the
// numbers come from Params, so the model is told the exact adjustment instead
// of inventing one. Each session keeps its phase RPE window: periodization and
// the guardrail still apply, so fatigue is handled through volume and load.
func adaptationInstruction(d *service.AdaptationDecision) string {
	p := d.Params
	switch d.Kind {
	case service.AdaptationIncreaseLoad:
		return fmt.Sprintf("%s. Progress: raise target_weight_kg of loaded main exercises by about %.1f%%; "+
			"keep sets, reps and each session's phase RPE window.", d.Reason, p.WeightPctChange*100)
	case service.AdaptationTriggerDeload:
		return fmt.Sprintf("%s. Unload: cut working sets by about %.0f%% and target_weight_kg by about %.0f%%; "+
			"in DELOAD-week sessions keep the sets (already unloaded) and only lower the weight; "+
			"keep target_rpe at the bottom of each session's phase window.",
			d.Reason, math.Abs(p.VolumePctChange)*100, math.Abs(p.WeightPctChange)*100)
	case service.AdaptationProposeExpressFormat:
		return d.Reason + ". Shorten: about 30 minutes per session with fewer exercises and sets; " +
			"keep the most important compound movement of each session."
	default:
		return d.Reason
	}
}
