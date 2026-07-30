package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
)

func TestInjury_CreationAndLifecycle(t *testing.T) {
	t.Run("Create valid injury", func(t *testing.T) {
		now := time.Now()
		inj, err := entity.NewInjury("inj-1", "Knee", "MODERATE", "Strain while squatting", now)
		require.NoError(t, err)

		assert.Equal(t, "inj-1", inj.ID())
		assert.Equal(t, "Knee", inj.MuscleGroup())
		assert.Equal(t, "MODERATE", inj.Severity())
		assert.Equal(t, "Strain while squatting", inj.Notes())
		assert.Equal(t, now, inj.ReportedAt())
		assert.False(t, inj.IsRecovered())
		assert.Nil(t, inj.RecoveredAt())
	})

	t.Run("Create injury with zero reportedAt defaults to current time", func(t *testing.T) {
		inj, err := entity.NewInjury("inj-zero", "Knee", "MILD", "", time.Time{})
		require.NoError(t, err)
		assert.False(t, inj.ReportedAt().IsZero())
	})

	t.Run("Create injury fails with empty ID or muscle group", func(t *testing.T) {
		_, err := entity.NewInjury("", "Knee", "MILD", "", time.Now())
		assert.Error(t, err)

		_, err = entity.NewInjury("inj-2", "", "MILD", "", time.Now())
		assert.Error(t, err)
	})

	t.Run("Recover injury lifecycle and zero recoveredAt", func(t *testing.T) {
		inj, err := entity.NewInjury("inj-3", "Shoulder", "SEVERE", "Rotator cuff pain", time.Now())
		require.NoError(t, err)

		recTime := time.Now().Add(24 * time.Hour)
		err = inj.Recover(recTime)
		require.NoError(t, err)

		assert.True(t, inj.IsRecovered())
		require.NotNil(t, inj.RecoveredAt())
		assert.Equal(t, recTime, *inj.RecoveredAt())

		// Duplicate recover fails
		err = inj.Recover(recTime)
		assert.ErrorIs(t, err, derror.ErrInjuryAlreadyClosed)

		// Zero recoveredAt defaults to now
		inj2, _ := entity.NewInjury("inj-4-zero", "Ankle", "MILD", "", time.Now())
		err = inj2.Recover(time.Time{})
		require.NoError(t, err)
		assert.True(t, inj2.IsRecovered())
		assert.False(t, inj2.RecoveredAt().IsZero())
	})

	t.Run("Reconstitute injury", func(t *testing.T) {
		now := time.Now()
		recTime := now.Add(time.Hour)
		inj := entity.ReconstituteInjury("inj-4", "Ankle", "MILD", "Twisted", now, true, &recTime)

		assert.Equal(t, "inj-4", inj.ID())
		assert.True(t, inj.IsRecovered())
		assert.Equal(t, &recTime, inj.RecoveredAt())
	})
}
