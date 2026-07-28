package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

// MotionSpecification is the aggregate root holding AI model URLs, pose rules, and audio scripts for an exercise.
type MotionSpecification struct {
	exerciseID             string
	onnxModelURL           string
	localRulesURL          string
	dialogueEngine         vo.DialogueEngineConfig
	recommendedCameraAngle string
	createdAt              time.Time
	updatedAt              time.Time
}

// NewMotionSpecification creates a new MotionSpecification.
func NewMotionSpecification(
	exerciseID, onnxModelURL, localRulesURL string,
	dialogueEngine vo.DialogueEngineConfig,
	recommendedCameraAngle string,
) *MotionSpecification {
	now := time.Now().UTC()
	return &MotionSpecification{
		exerciseID:             exerciseID,
		onnxModelURL:           onnxModelURL,
		localRulesURL:          localRulesURL,
		dialogueEngine:         dialogueEngine,
		recommendedCameraAngle: recommendedCameraAngle,
		createdAt:              now,
		updatedAt:              now,
	}
}

// ExerciseID returns the exercise ID.
func (m *MotionSpecification) ExerciseID() string { return m.exerciseID }

// OnnxModelURL returns the ONNX model URL.
func (m *MotionSpecification) OnnxModelURL() string { return m.onnxModelURL }

// LocalRulesURL returns the local rules URL.
func (m *MotionSpecification) LocalRulesURL() string { return m.localRulesURL }

// DialogueEngine returns the audio dialogue config.
func (m *MotionSpecification) DialogueEngine() vo.DialogueEngineConfig { return m.dialogueEngine }

// RecommendedCameraAngle returns recommended camera orientation.
func (m *MotionSpecification) RecommendedCameraAngle() string { return m.recommendedCameraAngle }
