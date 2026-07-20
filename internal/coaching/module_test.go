package coaching

import (
	"context"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

type nilExerciseService struct{}

func (*nilExerciseService) SearchExercises(
	context.Context,
	*port.ExerciseSearchFilters,
) ([]port.ExerciseInfo, error) {
	return nil, nil
}

func TestInitializeRequiresExerciseService(t *testing.T) {
	t.Parallel()

	_, err := Initialize(context.Background(), ModuleDeps{})
	if err == nil || !strings.Contains(err.Error(), "exercise query service is required") {
		t.Fatalf("error got = %v, want required exercise service", err)
	}
}

func TestInitializeRejectsTypedNilExerciseService(t *testing.T) {
	t.Parallel()

	var service *nilExerciseService
	_, err := Initialize(context.Background(), ModuleDeps{ExerciseService: service})
	if err == nil || !strings.Contains(err.Error(), "exercise query service is required") {
		t.Fatalf("error got = %v, want required exercise service", err)
	}
}
