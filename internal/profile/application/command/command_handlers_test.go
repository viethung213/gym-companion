package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/viethung213/gym-companion/internal/profile/application/command"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type failTxManager struct{}

func (f *failTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return errors.New("tx failed")
}

func TestSaveHealthProfileHandler_Errors(t *testing.T) {
	repo := newMockRepo()
	eventPub := &mockEventPub{}
	txManager := &mockTxManager{}

	handler := command.NewSaveHealthProfileHandler(repo, eventPub, txManager)

	// Invalid bio metrics (negative weight)
	cmd := command.SaveHealthProfileCommand{
		UserID:   "user-err-1",
		WeightKg: -10,
	}

	_, err := handler.Handle(context.Background(), cmd)
	assert.Error(t, err)

	// Invalid injury (empty muscle group)
	cmd2 := command.SaveHealthProfileCommand{
		UserID:   "user-err-2",
		WeightKg: 70.0,
		HeightCm: 170.0,
		Injuries: []command.InjuryInput{{ID: "inj-bad", MuscleGroup: ""}},
	}
	_, err = handler.Handle(context.Background(), cmd2)
	assert.Error(t, err)

	// Transaction error
	failHandler := command.NewSaveHealthProfileHandler(repo, eventPub, &failTxManager{})
	cmdSuccess := command.SaveHealthProfileCommand{UserID: "user-tx-fail", WeightKg: 70, HeightCm: 170, Age: 20, Gender: "MALE"}
	_, err = failHandler.Handle(context.Background(), cmdSuccess)
	assert.Error(t, err)
}

func TestUpdateProfileHandler_Errors(t *testing.T) {
	repo := newMockRepo()
	eventPub := &mockEventPub{}
	txManager := &mockTxManager{}

	handler := command.NewUpdateProfileHandler(repo, eventPub, txManager)

	// Profile not found
	cmd := command.UpdateProfileCommand{UserID: "non-existent"}
	err := handler.Handle(context.Background(), cmd)
	assert.ErrorIs(t, err, derror.ErrProfileNotFound)

	// Invalid bio metrics
	bio, _ := vo.NewBiologicalMetrics(70, 170, 20, "MALE")
	p, _ := aggregate.NewUserProfile("user-err-3", bio, "", nil, nil, nil, nil, "", 0, 0, nil)
	_ = repo.Save(context.Background(), p)

	cmd2 := command.UpdateProfileCommand{UserID: "user-err-3", WeightKg: -100}
	err = handler.Handle(context.Background(), cmd2)
	assert.Error(t, err)

	// Transaction error
	failHandler := command.NewUpdateProfileHandler(repo, eventPub, &failTxManager{})
	cmdValid := command.UpdateProfileCommand{UserID: "user-err-3", WeightKg: 70, HeightCm: 170}
	err = failHandler.Handle(context.Background(), cmdValid)
	assert.Error(t, err)
}

func TestLogPeriodicMetricsHandler(t *testing.T) {
	repo := newMockRepo()
	eventPub := &mockEventPub{}
	txManager := &mockTxManager{}

	bio, err := vo.NewBiologicalMetrics(70.0, 175.0, 25, "MALE")
	require.NoError(t, err)

	p, err := aggregate.NewUserProfile("user-cmd-1", bio, "BEGINNER", nil, nil, nil, nil, "", 0, 0, nil)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), p))

	handler := command.NewLogPeriodicMetricsHandler(repo, eventPub, txManager)

	// Profile not found
	_, err = handler.Handle(context.Background(), command.LogPeriodicMetricsCommand{UserID: "non-existent"})
	assert.ErrorIs(t, err, derror.ErrProfileNotFound)

	// Invalid metric (negative weight)
	_, err = handler.Handle(context.Background(), command.LogPeriodicMetricsCommand{UserID: "user-cmd-1", WeightKg: -5})
	assert.Error(t, err)

	// Success case
	cmd := command.LogPeriodicMetricsCommand{
		LogID:            "log-101",
		UserID:           "user-cmd-1",
		WeightKg:         72.5,
		BodyFatPercent:   16.0,
		ProgressPhotoURL: "https://photos.com/1.png",
	}

	res, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	assert.Equal(t, "log-101", res.LogID)
	assert.Equal(t, 72.5, res.WeightKg)
	assert.Equal(t, "SYNCED", res.SyncStatus)

	// Transaction error
	failHandler := command.NewLogPeriodicMetricsHandler(repo, eventPub, &failTxManager{})
	_, err = failHandler.Handle(context.Background(), cmd)
	assert.Error(t, err)
}

func TestReportAndRecoverInjuryHandler(t *testing.T) {
	repo := newMockRepo()
	eventPub := &mockEventPub{}
	txManager := &mockTxManager{}

	bio, err := vo.NewBiologicalMetrics(80.0, 180.0, 30, "MALE")
	require.NoError(t, err)

	p, err := aggregate.NewUserProfile("user-cmd-2", bio, "ADVANCED", nil, nil, nil, nil, "", 0, 0, nil)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), p))

	reportHandler := command.NewReportInjuryHandler(repo, eventPub, txManager)
	recoverHandler := command.NewRecoverInjuryHandler(repo, eventPub, txManager)

	// Report injury profile not found
	_, err = reportHandler.Handle(context.Background(), command.ReportInjuryCommand{UserID: "non-existent"})
	assert.ErrorIs(t, err, derror.ErrProfileNotFound)

	// Report invalid injury
	_, err = reportHandler.Handle(context.Background(), command.ReportInjuryCommand{UserID: "user-cmd-2", InjuryID: "inj-1", MuscleGroup: ""})
	assert.Error(t, err)

	// 1. Report Injury Success
	reportCmd := command.ReportInjuryCommand{
		InjuryID:    "inj-cmd-1",
		UserID:      "user-cmd-2",
		MuscleGroup: "Hamstring",
		Severity:    "MILD",
		Notes:       "Pulled while sprinting",
	}

	injuryID, err := reportHandler.Handle(context.Background(), reportCmd)
	require.NoError(t, err)
	assert.Equal(t, "inj-cmd-1", injuryID)

	// Transaction error on Report
	failReportHandler := command.NewReportInjuryHandler(repo, eventPub, &failTxManager{})
	_, err = failReportHandler.Handle(context.Background(), reportCmd)
	assert.Error(t, err)

	// Recover injury profile not found
	err = recoverHandler.Handle(context.Background(), command.RecoverInjuryCommand{UserID: "non-existent", InjuryID: "inj-cmd-1"})
	assert.ErrorIs(t, err, derror.ErrProfileNotFound)

	// 2. Recover Injury Success
	recoverCmd := command.RecoverInjuryCommand{
		UserID:   "user-cmd-2",
		InjuryID: "inj-cmd-1",
	}

	err = recoverHandler.Handle(context.Background(), recoverCmd)
	require.NoError(t, err)

	updatedProfile, err := repo.FindByUserID(context.Background(), "user-cmd-2")
	require.NoError(t, err)
	assert.Len(t, updatedProfile.Injuries(), 1)
	assert.True(t, updatedProfile.Injuries()[0].IsRecovered())

	// Transaction error on Recover
	failRecoverHandler := command.NewRecoverInjuryHandler(repo, eventPub, &failTxManager{})
	err = failRecoverHandler.Handle(context.Background(), recoverCmd)
	assert.Error(t, err)
}
