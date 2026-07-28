package aggregate_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

func TestMotionSpecification(t *testing.T) {
	dEngine := vo.DialogueEngineConfig{PersonalityID: "coach-1"}
	spec := aggregate.NewMotionSpecification("ex-1", "http://onnx.model", "http://rules.json", dEngine, "side")

	if got, want := spec.ExerciseID(), "ex-1"; got != want {
		t.Errorf("got ExerciseID = %v, want %v", got, want)
	}
	if got, want := spec.OnnxModelURL(), "http://onnx.model"; got != want {
		t.Errorf("got OnnxModelURL = %v, want %v", got, want)
	}
	if got, want := spec.LocalRulesURL(), "http://rules.json"; got != want {
		t.Errorf("got LocalRulesURL = %v, want %v", got, want)
	}
	if got, want := spec.RecommendedCameraAngle(), "side"; got != want {
		t.Errorf("got RecommendedCameraAngle = %v, want %v", got, want)
	}
	if got, want := spec.DialogueEngine().PersonalityID, "coach-1"; got != want {
		t.Errorf("got PersonalityID = %v, want %v", got, want)
	}
}
