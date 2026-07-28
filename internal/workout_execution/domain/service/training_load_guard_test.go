package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/service"
)

type mockVolumeProvider struct {
	volumes []float32
	err     error
}

func (m *mockVolumeProvider) GetRecentVolumesForMuscleGroup(ctx context.Context, userID, muscleGroup string, limit int) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.volumes, nil
}

func TestTrainingLoadGuard_Coverage(t *testing.T) {
	t.Run("nil history provider", func(t *testing.T) {
		guard := service.NewTrainingLoadGuard(nil)
		overloaded, avg, err := guard.IsOverloaded(context.Background(), "u1", "Chest", 500)
		if err != nil || overloaded || avg != 0 {
			t.Errorf("got %v, %v, %v; want false, 0, nil", overloaded, avg, err)
		}
	})

	t.Run("history provider returns error", func(t *testing.T) {
		guard := service.NewTrainingLoadGuard(&mockVolumeProvider{err: errors.New("db error")})
		_, _, err := guard.IsOverloaded(context.Background(), "u1", "Chest", 500)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("empty volume history", func(t *testing.T) {
		guard := service.NewTrainingLoadGuard(&mockVolumeProvider{volumes: []float32{}})
		overloaded, avg, err := guard.IsOverloaded(context.Background(), "u1", "Chest", 500)
		if err != nil || overloaded || avg != 0 {
			t.Errorf("got %v, %v, %v; want false, 0, nil", overloaded, avg, err)
		}
	})

	t.Run("normal load under 250%", func(t *testing.T) {
		guard := service.NewTrainingLoadGuard(&mockVolumeProvider{volumes: []float32{100, 100, 100}})
		overloaded, avg, err := guard.IsOverloaded(context.Background(), "u1", "Chest", 200)
		if err != nil || overloaded || avg != 100 {
			t.Errorf("got %v, %v, %v; want false, 100, nil", overloaded, avg, err)
		}
	})

	t.Run("overloaded over 250%", func(t *testing.T) {
		guard := service.NewTrainingLoadGuard(&mockVolumeProvider{volumes: []float32{100, 100, 100}})
		overloaded, avg, err := guard.IsOverloaded(context.Background(), "u1", "Chest", 300)
		if err != nil || !overloaded || avg != 100 {
			t.Errorf("got %v, %v, %v; want true, 100, nil", overloaded, avg, err)
		}
	})
}
