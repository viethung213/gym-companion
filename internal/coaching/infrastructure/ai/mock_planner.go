package ai

import (
	"context"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// MockPlanner implements port.WorkoutPlanner using a simple deterministic rule.
// To swap to Gemini SDK: create GeminiPlanner in this package implementing port.WorkoutPlanner
// and inject it in module.go constructors.
type MockPlanner struct{}

var _ port.WorkoutPlanner = (*MockPlanner)(nil)

// NewMockPlanner creates a new MockPlanner.
func NewMockPlanner() *MockPlanner {
	return &MockPlanner{}
}

// PlanWorkout arranges available exercises in order (compound first, isolation second).
func (m *MockPlanner) PlanWorkout(
	_ context.Context,
	req *port.PlanWorkoutRequest,
) (*port.PlanWorkoutResult, error) {
	if len(req.AvailableExercises) == 0 {
		return &port.PlanWorkoutResult{
			SelectedExerciseIDs:  nil,
			ReasoningExplanation: "Không tìm thấy bài tập phù hợp trong danh mục.",
		}, nil
	}

	// Simple selection: pick up to 4 available exercises
	limit := 4
	if len(req.AvailableExercises) < limit {
		limit = len(req.AvailableExercises)
	}

	selectedIDs := make([]string, limit)
	for i := 0; i < limit; i++ {
		selectedIDs[i] = req.AvailableExercises[i].ID
	}

	return &port.PlanWorkoutResult{
		SelectedExerciseIDs: selectedIDs,
		ReasoningExplanation: "Giáo án được sắp xếp theo nguyên tắc khoa học " +
			"thể thao (Mock AI Planner).",
	}, nil
}
