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

func TestMotionSpecificationUpdateSpecAndEvents(t *testing.T) {
	spec := aggregate.NewDraftMotionSpecification("ex-3", "http://detector.onnx", "http://skeleton.onnx")

	if spec.IsComplete() {
		t.Error("want IsComplete = false when rules and dialogue are empty")
	}

	spec.UpdateSpec(
		"http://detector-v2.onnx",
		"http://skeleton-v2.onnx",
		"http://rules-v2.json",
		"http://dialogue-v2.json",
		"front",
	)

	if !spec.IsReady() {
		t.Error("want IsReady = true after UpdateSpec with all URLs")
	}
	if !spec.IsComplete() {
		t.Error("want IsComplete = true after UpdateSpec with all URLs")
	}
	if spec.RecommendedCameraAngle() != "front" {
		t.Errorf("got camera angle %s, want front", spec.RecommendedCameraAngle())
	}

	events := spec.PopEvents()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if len(spec.PopEvents()) != 0 {
		t.Error("want 0 events after PopEvents")
	}
}

func TestMotionSpecification_UpdateSpec_PartialUpdates(t *testing.T) {
	spec := aggregate.RestoreMotionSpecification(
		"ex-4",
		"http://det1.onnx",
		"http://skel1.onnx",
		"http://rules1.json",
		"http://dialogue1.json",
		"side",
		true,
		time.Now().UTC().Add(-time.Hour),
		time.Now().UTC().Add(-time.Hour),
	)

	prevUpdatedAt := spec.UpdatedAt()
	time.Sleep(2 * time.Millisecond)

	// Partial update: empty strings should not overwrite existing URLs
	spec.UpdateSpec("", "", "http://rules2.json", "", "")

	if got, want := spec.OnnxDetectorURL(), "http://det1.onnx"; got != want {
		t.Errorf("got OnnxDetectorURL = %v, want %v", got, want)
	}
	if got, want := spec.OnnxSkeletonURL(), "http://skel1.onnx"; got != want {
		t.Errorf("got OnnxSkeletonURL = %v, want %v", got, want)
	}
	if got, want := spec.LocalRulesURL(), "http://rules2.json"; got != want {
		t.Errorf("got LocalRulesURL = %v, want %v", got, want)
	}
	if got, want := spec.DialogueEngineURL(), "http://dialogue1.json"; got != want {
		t.Errorf("got DialogueEngineURL = %v, want %v", got, want)
	}
	if got, want := spec.RecommendedCameraAngle(), "side"; got != want {
		t.Errorf("got RecommendedCameraAngle = %v, want %v", got, want)
	}

	if !spec.UpdatedAt().After(prevUpdatedAt) {
		t.Errorf("got UpdatedAt = %v, want after %v", spec.UpdatedAt(), prevUpdatedAt)
	}
}

func TestMotionSpecification_IsComplete(t *testing.T) {
	tests := []struct {
		name              string
		detectorURL       string
		skeletonURL       string
		localRulesURL     string
		dialogueEngineURL string
		want              bool
	}{
		{
			name:              "all URLs populated",
			detectorURL:       "http://det.onnx",
			skeletonURL:       "http://skel.onnx",
			localRulesURL:     "http://rules.json",
			dialogueEngineURL: "http://dialogue.json",
			want:              true,
		},
		{
			name:              "missing detector URL",
			detectorURL:       "",
			skeletonURL:       "http://skel.onnx",
			localRulesURL:     "http://rules.json",
			dialogueEngineURL: "http://dialogue.json",
			want:              false,
		},
		{
			name:              "missing skeleton URL",
			detectorURL:       "http://det.onnx",
			skeletonURL:       "",
			localRulesURL:     "http://rules.json",
			dialogueEngineURL: "http://dialogue.json",
			want:              false,
		},
		{
			name:              "missing local rules URL",
			detectorURL:       "http://det.onnx",
			skeletonURL:       "http://skel.onnx",
			localRulesURL:     "",
			dialogueEngineURL: "http://dialogue.json",
			want:              false,
		},
		{
			name:              "missing dialogue engine URL",
			detectorURL:       "http://det.onnx",
			skeletonURL:       "http://skel.onnx",
			localRulesURL:     "http://rules.json",
			dialogueEngineURL: "",
			want:              false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			spec := aggregate.RestoreMotionSpecification(
				"ex-test",
				tt.detectorURL,
				tt.skeletonURL,
				tt.localRulesURL,
				tt.dialogueEngineURL,
				"front",
				false,
				time.Now().UTC(),
				time.Now().UTC(),
			)
			if got := spec.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMotionSpecification_PopEvents(t *testing.T) {
	spec := aggregate.NewDraftMotionSpecification("ex-pop", "http://det.onnx", "http://skel.onnx")
	spec.UpdateSpec("http://det2.onnx", "http://skel2.onnx", "http://rules.json", "http://dialogue.json", "front")

	evs1 := spec.PopEvents()
	if got, want := len(evs1), 1; got != want {
		t.Fatalf("first PopEvents() count got %d, want %d", got, want)
	}

	evs2 := spec.PopEvents()
	if got, want := len(evs2), 0; got != want {
		t.Errorf("second PopEvents() count got %d, want %d", got, want)
	}
}
