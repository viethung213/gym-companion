//go:build unit

package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/domain/service"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

func TestCompletionCalculator(t *testing.T) {
	calc := service.NewCompletionCalculator()

	t.Run("Fully completed profile gives 100%", func(t *testing.T) {
		bio, err := vo.NewBiologicalMetrics(70.0, 175.0, 25, "MALE")
		require.NoError(t, err)

		res := calc.Calculate(
			bio,
			"INTERMEDIATE",
			[]string{"MUSCLE_GAIN"},
			[]string{"MORNING"},
			[]string{"DUMBBELL"},
			[]string{"CHEST"},
			"FRIENDLY",
			75.0,
			nil,
		)

		assert.InDelta(t, 100.0, res.CompletionRate, 0.01)
		assert.True(t, res.AICoachActivated)
	})

	t.Run("8 out of 10 criteria gives 80% and activates AI Coach", func(t *testing.T) {
		bio, err := vo.NewBiologicalMetrics(70.0, 175.0, 25, "MALE")
		require.NoError(t, err)

		res := calc.Calculate(
			bio,
			"BEGINNER",
			[]string{"FAT_LOSS"},
			[]string{"EVENING"},
			[]string{}, // missing equipment
			[]string{}, // missing preferred muscle groups
			"STRICT",
			0.0,
			nil,
		)

		assert.InDelta(t, 80.0, res.CompletionRate, 0.01)
		assert.True(t, res.AICoachActivated)
	})

	t.Run("Empty profile gives 0% and does not activate AI Coach", func(t *testing.T) {
		bio, _ := vo.NewBiologicalMetrics(0, 0, 0, "")
		res := calc.Calculate(
			bio,
			"",
			nil,
			nil,
			nil,
			nil,
			"",
			0.0,
			nil,
		)

		assert.Equal(t, 0.0, res.CompletionRate)
		assert.False(t, res.AICoachActivated)
	})
}
