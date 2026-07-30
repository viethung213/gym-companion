package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
)

// MotionSpecification is the aggregate root holding AI model URLs, pose rules, and audio scripts for an exercise.
type MotionSpecification struct {
	exerciseID             string
	onnxModelURL           string
	localRulesURL          string
	dialogueEngineURL      string
	recommendedCameraAngle string
	isReady                bool
	createdAt              time.Time
	updatedAt              time.Time
	events                 []interface{}
}

// NewDraftMotionSpecification creates an initial draft (is_ready = false) when an ExerciseCreated event is consumed.
func NewDraftMotionSpecification(exerciseID string) *MotionSpecification {
	now := time.Now().UTC()
	return &MotionSpecification{
		exerciseID: exerciseID,
		isReady:    false,
		createdAt:  now,
		updatedAt:  now,
	}
}

// RestoreMotionSpecification restores a persisted MotionSpecification from its stored fields (used by repository).
func RestoreMotionSpecification(
	exerciseID, onnxModelURL, localRulesURL, dialogueEngineURL string,
	recommendedCameraAngle string,
	isReady bool,
	createdAt, updatedAt time.Time,
) *MotionSpecification {
	return &MotionSpecification{
		exerciseID:             exerciseID,
		onnxModelURL:           onnxModelURL,
		localRulesURL:          localRulesURL,
		dialogueEngineURL:      dialogueEngineURL,
		recommendedCameraAngle: recommendedCameraAngle,
		isReady:                isReady,
		createdAt:              createdAt,
		updatedAt:              updatedAt,
	}
}

// UpdateSpec updates the motion specification details, recomputes is_ready, and records a MotionSpecificationUpdated domain event.
func (m *MotionSpecification) UpdateSpec(
	onnxModelURL, localRulesURL, dialogueEngineURL string,
	recommendedCameraAngle string,
) {
	if onnxModelURL != "" {
		m.onnxModelURL = onnxModelURL
	}
	if localRulesURL != "" {
		m.localRulesURL = localRulesURL
	}
	if dialogueEngineURL != "" {
		m.dialogueEngineURL = dialogueEngineURL
	}
	if recommendedCameraAngle != "" {
		m.recommendedCameraAngle = recommendedCameraAngle
	}
	m.isReady = m.IsComplete()
	m.updatedAt = time.Now().UTC()

	m.events = append(m.events, event.MotionSpecificationUpdated{
		ExerciseID:             m.exerciseID,
		OnnxModelURL:           m.onnxModelURL,
		LocalRulesURL:          m.localRulesURL,
		DialogueEngineURL:      m.dialogueEngineURL,
		RecommendedCameraAngle: m.recommendedCameraAngle,
		IsReady:                m.isReady,
		UpdatedAt:              m.updatedAt,
	})
}

// IsComplete returns true when ONNX model URL, local rules URL, and dialogue engine URL are all present.
func (m *MotionSpecification) IsComplete() bool {
	return m.onnxModelURL != "" && m.localRulesURL != "" && m.dialogueEngineURL != ""
}

// PopEvents returns and clears recorded domain events.
func (m *MotionSpecification) PopEvents() []interface{} {
	evs := m.events
	m.events = nil
	return evs
}

// ExerciseID returns the exercise ID.
func (m *MotionSpecification) ExerciseID() string { return m.exerciseID }

// OnnxModelURL returns the ONNX model URL.
func (m *MotionSpecification) OnnxModelURL() string { return m.onnxModelURL }

// LocalRulesURL returns the local rules URL.
func (m *MotionSpecification) LocalRulesURL() string { return m.localRulesURL }

// DialogueEngineURL returns the audio dialogue config S3 URL.
func (m *MotionSpecification) DialogueEngineURL() string { return m.dialogueEngineURL }

// RecommendedCameraAngle returns recommended camera orientation.
func (m *MotionSpecification) RecommendedCameraAngle() string { return m.recommendedCameraAngle }

// IsReady returns whether this specification is fully configured and ready for AI-powered sessions.
func (m *MotionSpecification) IsReady() bool { return m.isReady }

// CreatedAt returns creation timestamp.
func (m *MotionSpecification) CreatedAt() time.Time { return m.createdAt }

// UpdatedAt returns update timestamp.
func (m *MotionSpecification) UpdatedAt() time.Time { return m.updatedAt }
