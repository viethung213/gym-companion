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
	domainEvent "github.com/viethung213/gym-companion/internal/profile/domain/event"
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
			[]vo.Equipment{vo.EquipmentDumbbell, vo.EquipmentOther},
			[]vo.MuscleGroup{vo.MuscleGroupChest, vo.MuscleGroupShoulders},
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
		assert.Equal(t, []vo.Equipment{vo.EquipmentDumbbell, vo.EquipmentOther}, p.AvailableEquipment())
		assert.Equal(t, []vo.MuscleGroup{vo.MuscleGroupChest, vo.MuscleGroupShoulders}, p.PreferredMuscleGroups())
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
			[]vo.Equipment{vo.EquipmentDumbbell},
			[]vo.MuscleGroup{vo.MuscleGroupQuadriceps},
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
	require.NoError(t, p.UpdateProfile(
		updatedBio,
		"INTERMEDIATE",
		[]string{"FAT_LOSS"},
		[]string{"MORNING"},
		[]vo.Equipment{vo.EquipmentDumbbell},
		[]vo.MuscleGroup{vo.MuscleGroupChest},
		"STRICT",
		65.0,
		15.0,
	))

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
		[]vo.Equipment{vo.EquipmentDumbbell},
		[]vo.MuscleGroup{vo.MuscleGroupQuadriceps},
		"STRICT",
		75.0,
		12.0,
		nil,
	)
	require.NoError(t, err)
	_ = p.PopEvents() // clear initial events

	inj, err := entity.NewInjury("inj-1", vo.JointRegionKnee, "", vo.BodySideBoth, vo.InjurySeverityModerate, "Slight strain", time.Now())
	require.NoError(t, err)

	err = p.AddInjury(inj)
	require.NoError(t, err)
	assert.Len(t, p.Injuries(), 1)
	assert.False(t, p.Injuries()[0].IsRecovered())

	events := p.PopEvents()
	assert.Len(t, events, 1) // InjuryReportedEvent

	// Attempt adding duplicate active injury for same muscle group
	inj2, err := entity.NewInjury("inj-2", vo.JointRegionKnee, "", vo.BodySideBoth, vo.InjurySeveritySevere, "Twisted again", time.Now())
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
	inj, err := entity.NewInjury("inj-1", vo.JointRegionKnee, "", vo.BodySideBoth, vo.InjurySeverityMild, "Test strain", time.Now())
	require.NoError(t, err)

	p, err := aggregate.NewUserProfile(
		"user-def-1",
		bio,
		"INTERMEDIATE",
		goals,
		[]string{"MORNING"},
		[]vo.Equipment{vo.EquipmentBarbell},
		[]vo.MuscleGroup{vo.MuscleGroupChest},
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

func TestUserProfile_UpdateIdentity(t *testing.T) {
	bio, _ := vo.NewBiologicalMetrics(70, 170, 25, "MALE")

	t.Run("Successfully update both full name and avatar url", func(t *testing.T) {
		p, err := aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		require.NoError(t, err)
		p.PopEvents() // clear creation events

		newName := "Nguyen Van A"
		newAvatar := "https://example.com/avatar.jpg"
		err = p.UpdateIdentity(&newName, &newAvatar)
		require.NoError(t, err)
		assert.Equal(t, newName, p.FullName())
		assert.Equal(t, newAvatar, p.AvatarURL())

		events := p.PopEvents()
		require.Len(t, events, 1)
		ev, ok := events[0].(*domainEvent.UserIdentityUpdatedEvent)
		require.True(t, ok)
		assert.Equal(t, "user-1", ev.UserID())
		assert.Equal(t, newName, ev.FullName())
		assert.Equal(t, newAvatar, ev.AvatarURL())
		assert.False(t, ev.UpdatedAt().IsZero())
	})

	t.Run("Update only full name", func(t *testing.T) {
		p, err := aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		require.NoError(t, err)
		p.SetIdentity("Old Name", "https://old.com/avatar.jpg")
		p.PopEvents()

		newName := "New Name"
		err = p.UpdateIdentity(&newName, nil)
		require.NoError(t, err)
		assert.Equal(t, "New Name", p.FullName())
		assert.Equal(t, "https://old.com/avatar.jpg", p.AvatarURL())

		events := p.PopEvents()
		require.Len(t, events, 1)
	})

	t.Run("Update only avatar url", func(t *testing.T) {
		p, err := aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		require.NoError(t, err)
		p.SetIdentity("Old Name", "https://old.com/avatar.jpg")
		p.PopEvents()

		newAvatar := "https://new.com/avatar.jpg"
		err = p.UpdateIdentity(nil, &newAvatar)
		require.NoError(t, err)
		assert.Equal(t, "Old Name", p.FullName())
		assert.Equal(t, newAvatar, p.AvatarURL())

		events := p.PopEvents()
		require.Len(t, events, 1)
	})

	t.Run("Error when both arguments are nil", func(t *testing.T) {
		p, err := aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		require.NoError(t, err)

		err = p.UpdateIdentity(nil, nil)
		assert.ErrorIs(t, err, derror.ErrEmptyIdentityUpdate)
	})

	t.Run("Error when full name exceeds 100 characters", func(t *testing.T) {
		p, err := aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		require.NoError(t, err)

		longName := ""
		for i := 0; i < 101; i++ {
			longName += "a"
		}
		err = p.UpdateIdentity(&longName, nil)
		assert.ErrorIs(t, err, derror.ErrInvalidFullName)
	})

	t.Run("No event recorded when values are unchanged", func(t *testing.T) {
		p, err := aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		require.NoError(t, err)
		p.SetIdentity("Current Name", "https://current.com/avatar.jpg")
		p.PopEvents()

		sameName := "Current Name"
		sameAvatar := "https://current.com/avatar.jpg"
		err = p.UpdateIdentity(&sameName, &sameAvatar)
		require.NoError(t, err)
		events := p.PopEvents()
		assert.Empty(t, events)
	})

	t.Run("Update avatar with empty string resets to DefaultAvatarURL", func(t *testing.T) {
		p, err := aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		require.NoError(t, err)
		p.SetIdentity("Current Name", "https://custom.com/avatar.jpg")
		p.PopEvents()

		emptyAvatar := ""
		err = p.UpdateIdentity(nil, &emptyAvatar)
		require.NoError(t, err)
		assert.Equal(t, aggregate.DefaultAvatarURL, p.AvatarURL())

		events := p.PopEvents()
		require.Len(t, events, 1)
		ev, ok := events[0].(*domainEvent.UserIdentityUpdatedEvent)
		require.True(t, ok)
		assert.Equal(t, aggregate.DefaultAvatarURL, ev.AvatarURL())
	})

	t.Run("NewUserProfile and SetIdentity default avatar URL", func(t *testing.T) {
		p, err := aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
		require.NoError(t, err)
		assert.Equal(t, aggregate.DefaultAvatarURL, p.AvatarURL())

		p.SetIdentity("User Name", "")
		assert.Equal(t, aggregate.DefaultAvatarURL, p.AvatarURL())
		assert.Equal(t, "User Name", p.FullName())
	})
}

func TestUserProfile_RejectsUnknownTrainingCodes(t *testing.T) {
	bio, err := vo.NewBiologicalMetrics(70, 170, 30, "MALE")
	require.NoError(t, err)

	_, err = aggregate.NewUserProfile("user-x", bio, "BEGINNER", nil, nil,
		[]vo.Equipment{"bench"}, nil, "", 0, 0, nil)
	assert.ErrorIs(t, err, derror.ErrInvalidTaxonomy)

	p, err := aggregate.NewUserProfile("user-x", bio, "BEGINNER", nil, nil,
		[]vo.Equipment{vo.EquipmentDumbbell}, nil, "", 0, 0, nil)
	require.NoError(t, err)

	err = p.UpdateProfile(bio, "", nil, nil, nil, []vo.MuscleGroup{"LEGS"}, "", 0, 0)
	assert.ErrorIs(t, err, derror.ErrInvalidTaxonomy)
	assert.Empty(t, p.PreferredMuscleGroups(), "a rejected update must not change the profile")
}

func TestUserProfile_AddInjuryDuplicateIsPerLocation(t *testing.T) {
	bio, err := vo.NewBiologicalMetrics(70, 170, 30, "MALE")
	require.NoError(t, err)
	p, err := aggregate.NewUserProfile("user-y", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
	require.NoError(t, err)

	newInjury := func(id string, side vo.BodySide) *entity.Injury {
		inj, injErr := entity.NewInjury(id, vo.JointRegionKnee, "", side, vo.InjurySeverityMild, "", time.Now())
		require.NoError(t, injErr)
		return inj
	}

	require.NoError(t, p.AddInjury(newInjury("left", vo.BodySideLeft)))
	assert.NoError(t, p.AddInjury(newInjury("right", vo.BodySideRight)), "other side is a separate injury")
	assert.ErrorIs(t, p.AddInjury(newInjury("left-again", vo.BodySideLeft)), derror.ErrInjuryAlreadyActive)
}
