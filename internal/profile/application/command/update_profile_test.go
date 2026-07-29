package command_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

func TestUpdateProfileHandler_FiresEventOnReaching80Percent(t *testing.T) {
	repo := newMockRepo()
	eventPub := &mockEventPub{}
	txManager := &mockTxManager{}

	// 1. Create initial profile with low completion rate (< 80%)
	bioIncomplete, _ := vo.NewBiologicalMetrics(70.0, 170.0, 25, "MALE")
	initialProfile, err := aggregate.NewUserProfile(
		"user-update-80",
		bioIncomplete,
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
	assert.False(t, initialProfile.AICoachActivated())
	require.NoError(t, repo.Save(context.Background(), initialProfile))

	// Clear initial events
	_ = initialProfile.PopEvents()

	// 2. Update profile adding experience, goals, preferred times, equipment, muscle groups, coach style to cross >= 80%
	updateHandler := command.NewUpdateProfileHandler(repo, eventPub, txManager)
	cmd := command.UpdateProfileCommand{
		UserID:                "user-update-80",
		WeightKg:              70.0,
		HeightCm:              170.0,
		DateOfBirth:           "1998-05-15",
		Age:                   25,
		Gender:                "MALE",
		Goals:                 []string{"FAT_LOSS"},
		ExperienceLevel:       "INTERMEDIATE",
		PreferredWorkoutTimes: []string{"MORNING"},
		AvailableEquipment:    []string{"DUMBBELL"},
		PreferredMuscleGroups: []string{"CHEST"},
		CoachStyle:            "FRIENDLY",
		TargetWeightKg:        65.0,
		TargetBodyFatPercent:  15.0,
	}

	err = updateHandler.Handle(context.Background(), cmd)
	require.NoError(t, err)

	updatedProfile, err := repo.FindByUserID(context.Background(), "user-update-80")
	require.NoError(t, err)
	assert.True(t, updatedProfile.AICoachActivated())
	assert.GreaterOrEqual(t, updatedProfile.CompletionRate(), 80.0)

	// Verify ProfileUpdatedEvent & ProfileCompletedEvent were fired and published to Outbox
	assert.Len(t, eventPub.published, 2)
}
