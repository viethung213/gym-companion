//go:build unit

package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

func TestInjury_CreationAndLifecycle(t *testing.T) {
	t.Run("Create valid injury", func(t *testing.T) {
		now := time.Now()
		inj, err := entity.NewInjury("inj-1", vo.JointRegionKnee, "", vo.BodySideBoth, vo.InjurySeverityModerate, "Strain while squatting", now)
		require.NoError(t, err)

		assert.Equal(t, "inj-1", inj.ID())
		assert.Equal(t, vo.JointRegionKnee, inj.Region())
		assert.Equal(t, vo.MuscleGroup(""), inj.Muscle())
		assert.Equal(t, vo.BodySideBoth, inj.Side())
		assert.Equal(t, vo.InjurySeverityModerate, inj.Severity())
		assert.Equal(t, "Strain while squatting", inj.Notes())
		assert.Equal(t, now, inj.ReportedAt())
		assert.False(t, inj.IsRecovered())
		assert.Nil(t, inj.RecoveredAt())
	})

	t.Run("Create injury with zero reportedAt defaults to current time", func(t *testing.T) {
		inj, err := entity.NewInjury("inj-zero", vo.JointRegionKnee, "", vo.BodySideBoth, vo.InjurySeverityMild, "", time.Time{})
		require.NoError(t, err)
		assert.False(t, inj.ReportedAt().IsZero())
	})

	t.Run("Create injury fails with empty ID", func(t *testing.T) {
		_, err := entity.NewInjury("", vo.JointRegionKnee, "", vo.BodySideBoth, vo.InjurySeverityMild, "", time.Now())
		assert.Error(t, err)
	})

	t.Run("Create injury validates taxonomy", func(t *testing.T) {
		tests := []struct {
			name     string
			region   vo.JointRegion
			muscle   vo.MuscleGroup
			side     vo.BodySide
			severity vo.InjurySeverity
		}{
			{"missing region", "", "", vo.BodySideBoth, vo.InjurySeverityMild},
			{"free-text region", "Knee", "", vo.BodySideBoth, vo.InjurySeverityMild},
			{"unknown muscle", vo.JointRegionKnee, "QUADS", vo.BodySideBoth, vo.InjurySeverityMild},
			{"unknown side", vo.JointRegionKnee, "", "TOP", vo.InjurySeverityMild},
			{"missing severity", vo.JointRegionKnee, "", vo.BodySideBoth, ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := entity.NewInjury("inj-x", tt.region, tt.muscle, tt.side, tt.severity, "", time.Now())
				assert.ErrorIs(t, err, derror.ErrInvalidTaxonomy)
			})
		}
	})

	t.Run("Create injury with muscle and default side", func(t *testing.T) {
		inj, err := entity.NewInjury("inj-m", vo.JointRegionKnee, vo.MuscleGroupQuadriceps, "", vo.InjurySeverityMild, "", time.Now())
		require.NoError(t, err)
		assert.Equal(t, vo.MuscleGroupQuadriceps, inj.Muscle())
		assert.Equal(t, vo.BodySideBoth, inj.Side())
	})

	t.Run("Recover injury lifecycle and zero recoveredAt", func(t *testing.T) {
		inj, err := entity.NewInjury("inj-3", vo.JointRegionShoulder, "", vo.BodySideBoth, vo.InjurySeveritySevere, "Rotator cuff pain", time.Now())
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
		inj2, _ := entity.NewInjury("inj-4-zero", vo.JointRegionAnkleFoot, "", vo.BodySideBoth, vo.InjurySeverityMild, "", time.Now())
		err = inj2.Recover(time.Time{})
		require.NoError(t, err)
		assert.True(t, inj2.IsRecovered())
		assert.False(t, inj2.RecoveredAt().IsZero())
	})

	t.Run("Reconstitute injury", func(t *testing.T) {
		now := time.Now()
		recTime := now.Add(time.Hour)
		inj := entity.ReconstituteInjury("inj-4", vo.JointRegionAnkleFoot, "", vo.BodySideBoth, vo.InjurySeverityMild, "Twisted", now, true, &recTime)

		assert.Equal(t, "inj-4", inj.ID())
		assert.True(t, inj.IsRecovered())
		assert.Equal(t, &recTime, inj.RecoveredAt())
	})
}
