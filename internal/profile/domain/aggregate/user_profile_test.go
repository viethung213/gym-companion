//go:build unit

package aggregate_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

func TestUserProfile_NewUserProfile(t *testing.T) {
	t.Run("Create complete profile - AI coach activated", func(t *testing.T) {
		bio, err := vo.NewBiologicalMetrics(75.5, 178.0, 25, "MALE")
		require.NoError(t, err)

		injuries := []*entity.Injury{}
		p, err := aggregate.NewUserProfile(
			"user-123",
			bio,
			"INTERMEDIATE",
			[]string{"PRIMARY_GOAL_MUSCLE_GAIN"},
			[]string{"MORNING"},
			[]string{"DUMBBELL", "MAT"},
			[]string{"CHEST", "SHOULDERS"},
			"FRIENDLY",
			70.0,
			15.0,
			injuries,
		)
		require.NoError(t, err)
		assert.Equal(t, "user-123", p.UserID())
		assert.True(t, p.AICoachActivated())
		assert.GreaterOrEqual(t, p.CompletionRate(), 80.0)

		events := p.PopEvents()
		assert.Len(t, events, 2) // ProfileUpdatedEvent & ProfileCompletedEvent

		// Test getters
		assert.Equal(t, bio, p.BiologicalMetrics())
		assert.Equal(t, "INTERMEDIATE", p.ExperienceLevel())
		assert.Equal(t, []string{"PRIMARY_GOAL_MUSCLE_GAIN"}, p.Goals())
		assert.Equal(t, []string{"MORNING"}, p.PreferredWorkoutTimes())
		assert.Equal(t, []string{"DUMBBELL", "MAT"}, p.AvailableEquipment())
		assert.Equal(t, []string{"CHEST", "SHOULDERS"}, p.PreferredMuscleGroups())
		assert.Equal(t, "FRIENDLY", p.CoachStyle())
		assert.Equal(t, 70.0, p.TargetWeightKg())
		assert.Equal(t, 15.0, p.TargetBodyFatPercent())
		assert.False(t, p.CreatedAt().IsZero())
		assert.False(t, p.UpdatedAt().IsZero())
	})

	t.Run("Create profile fails with empty UserID", func(t *testing.T) {
		bio, _ := vo.NewBiologicalMetrics(70, 170, 20, "MALE")
		_, err := aggregate.NewUserProfile("", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		assert.Error(t, err)
	})

	t.Run("Create incomplete profile - AI coach not activated", func(t *testing.T) {
		bio, err := vo.NewBiologicalMetrics(70.0, 170.0, 20, "MALE")
		require.NoError(t, err)

		p, err := aggregate.NewUserProfile(
			"user-456",
			bio,
			"",
			nil,
			nil,
			nil,
			nil,
			"",
			0,
			0,
			nil,
		)
		require.NoError(t, err)
		assert.False(t, p.AICoachActivated())
		assert.Less(t, p.CompletionRate(), 80.0)
	})

	t.Run("ReconstituteUserProfile", func(t *testing.T) {
		bio, _ := vo.NewBiologicalMetrics(70, 175, 25, "MALE")
		now := time.Now()
		p := aggregate.ReconstituteUserProfile(
			"user-recon-1",
			bio,
			"BEGINNER",
			[]string{"FAT_LOSS"},
			[]string{"EVENING"},
			[]string{"DUMBBELL"},
			[]string{"LEGS"},
			"STRICT",
			65.0,
			15.0,
			nil,
			nil,
			80.0,
			true,
			now,
			now,
		)
		assert.Equal(t, "user-recon-1", p.UserID())
		assert.True(t, p.AICoachActivated())
	})
}

func TestUserProfile_UpdateAndMetrics(t *testing.T) {
	bio, _ := vo.NewBiologicalMetrics(70.0, 170.0, 20, "MALE")
	p, err := aggregate.NewUserProfile("user-upd-1", bio, "", nil, nil, nil, nil, "", 0, 0, nil)
	require.NoError(t, err)
	_ = p.PopEvents()

	// UpdateProfile crossing threshold >= 80%
	updatedBio, _ := vo.NewBiologicalMetrics(70.0, 170.0, 20, "MALE")
	p.UpdateProfile(
		updatedBio,
		"INTERMEDIATE",
		[]string{"FAT_LOSS"},
		[]string{"MORNING"},
		[]string{"DUMBBELL"},
		[]string{"CHEST"},
		"STRICT",
		65.0,
		15.0,
	)

	assert.True(t, p.AICoachActivated())
	events := p.PopEvents()
	assert.Len(t, events, 2) // ProfileUpdatedEvent & ProfileCompletedEvent

	// Add Periodic Metric
	pm, _ := vo.NewPeriodicMetric("pm-10", 69.0, 14.0, "", time.Now())
	p.AddPeriodicMetric(pm)
	assert.Len(t, p.PeriodicMetrics(), 2)
}

func TestUserProfile_InjuryLifecycle(t *testing.T) {
	bio, err := vo.NewBiologicalMetrics(80.0, 180.0, 30, "MALE")
	require.NoError(t, err)

	p, err := aggregate.NewUserProfile(
		"user-789",
		bio,
		"ADVANCED",
		[]string{"STRENGTH"},
		[]string{"EVENING"},
		[]string{"DUMBBELL"},
		[]string{"LEGS"},
		"STRICT",
		75.0,
		12.0,
		nil,
	)
	require.NoError(t, err)
	_ = p.PopEvents() // clear initial events

	inj, err := entity.NewInjury("inj-1", "Knee", "MODERATE", "Slight strain", time.Now())
	require.NoError(t, err)

	err = p.AddInjury(inj)
	require.NoError(t, err)
	assert.Len(t, p.Injuries(), 1)
	assert.False(t, p.Injuries()[0].IsRecovered())

	events := p.PopEvents()
	assert.Len(t, events, 1) // InjuryReportedEvent

	// Attempt adding duplicate active injury for same muscle group
	inj2, err := entity.NewInjury("inj-2", "Knee", "SEVERE", "Twisted again", time.Now())
	require.NoError(t, err)
	err = p.AddInjury(inj2)
	assert.ErrorIs(t, err, derror.ErrInjuryAlreadyActive)

	// Recover non-existent injury
	err = p.RecoverInjury("inj-999", time.Now())
	assert.ErrorIs(t, err, derror.ErrInjuryNotFound)

	// Recover injury successfully
	err = p.RecoverInjury("inj-1", time.Now())
	require.NoError(t, err)
	assert.True(t, p.Injuries()[0].IsRecovered())

	// Recover already recovered injury
	err = p.RecoverInjury("inj-1", time.Now())
	assert.ErrorIs(t, err, derror.ErrInjuryAlreadyClosed)
}

func TestUserProfile_DefensiveCopy(t *testing.T) {
	bio, err := vo.NewBiologicalMetrics(70.0, 175.0, 25, "MALE")
	require.NoError(t, err)

	goals := []string{"STRENGTH"}
	inj, err := entity.NewInjury("inj-1", "Knee", "MILD", "Test strain", time.Now())
	require.NoError(t, err)

	p, err := aggregate.NewUserProfile(
		"user-def-1",
		bio,
		"INTERMEDIATE",
		goals,
		[]string{"MORNING"},
		[]string{"BARBELL"},
		[]string{"CHEST"},
		"STRICT",
		70.0,
		15.0,
		[]*entity.Injury{inj},
	)
	require.NoError(t, err)

	// 1. Mutate input slice after creation
	goals[0] = "MUTATED_INPUT"
	assert.Equal(t, []string{"STRENGTH"}, p.Goals())

	// 2. Mutate output slice returned by getter
	retGoals := p.Goals()
	retGoals[0] = "MUTATED_OUTPUT"
	assert.Equal(t, []string{"STRENGTH"}, p.Goals())

	// 3. Mutate returned injury object pointer
	retInjuries := p.Injuries()
	require.Len(t, retInjuries, 1)
	err = retInjuries[0].Recover(time.Now())
	require.NoError(t, err)
	// Internal injury in aggregate must remain NOT recovered until RecoverInjury method is called on aggregate
	assert.False(t, p.Injuries()[0].IsRecovered())
}
