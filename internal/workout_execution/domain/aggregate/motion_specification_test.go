package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
)

func TestMotionSpecification(t *testing.T) {

	now := time.Now().UTC()

	spec := aggregate.RestoreMotionSpecification("ex-1", "http://detector.onnx", "http://skeleton.onnx", "http://rules.json", "http://dialogue.json", "side", true, now, now)

	if got, want := spec.ExerciseID(), "ex-1"; got != want {

		t.Errorf("got ExerciseID = %v, want %v", got, want)

	}

	if got, want := spec.OnnxDetectorURL(), "http://detector.onnx"; got != want {

		t.Errorf("got OnnxDetectorURL = %v, want %v", got, want)

	}

	if got, want := spec.OnnxSkeletonURL(), "http://skeleton.onnx"; got != want {

		t.Errorf("got OnnxSkeletonURL = %v, want %v", got, want)

	}

	if got, want := spec.LocalRulesURL(), "http://rules.json"; got != want {

		t.Errorf("got LocalRulesURL = %v, want %v", got, want)

	}

	if got, want := spec.DialogueEngineURL(), "http://dialogue.json"; got != want {

		t.Errorf("got DialogueEngineURL = %v, want %v", got, want)

	}

	if got, want := spec.RecommendedCameraAngle(), "side"; got != want {

		t.Errorf("got RecommendedCameraAngle = %v, want %v", got, want)

	}

	if !spec.IsReady() {

		t.Error("want IsReady = true for spec with all URLs set")

	}

}

func TestNewDraftMotionSpecification(t *testing.T) {

	spec := aggregate.NewDraftMotionSpecification("ex-2", "", "")

	if spec.ExerciseID() != "ex-2" {

		t.Errorf("got ExerciseID = %v, want ex-2", spec.ExerciseID())

	}

	if spec.IsReady() {

		t.Error("want IsReady = false for a new draft")

	}

}
