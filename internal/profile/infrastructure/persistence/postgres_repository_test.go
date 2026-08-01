package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/persistence"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	err = db.Exec("ATTACH DATABASE ':memory:' AS profile;").Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE profile.users (
			user_id TEXT PRIMARY KEY,
			date_of_birth DATETIME,
			gender TEXT,
			experience_level TEXT,
			goals BLOB,
			preferred_workout_times BLOB,
			available_equipment BLOB,
			preferred_muscle_groups BLOB,
			coach_style TEXT,
			target_weight_kg REAL,
			target_body_fat_percent REAL,
			completion_rate REAL,
			ai_coach_activated INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE profile.body_metrics (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			weight_kg REAL,
			height_cm REAL,
			body_fat_percent REAL,
			progress_photo_url TEXT,
			logged_at DATETIME
		);
		CREATE TABLE profile.injuries (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			muscle_group TEXT NOT NULL,
			severity TEXT NOT NULL,
			notes TEXT,
			reported_at DATETIME,
			is_recovered INTEGER,
			recovered_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE profile.outbox (
			id TEXT PRIMARY KEY,
			event_id TEXT UNIQUE NOT NULL,
			event_type TEXT NOT NULL,
			payload BLOB NOT NULL,
			partition_key TEXT NOT NULL,
			created_at DATETIME,
			published INTEGER DEFAULT 0,
			published_at DATETIME
		);
	`).Error
	require.NoError(t, err)

	return db
}

func TestPostgresUserProfileRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := persistence.NewPostgresUserProfileRepository(db)
	outboxRepo := persistence.NewGormOutboxRepository(db)
	txManager := persistence.NewSQLTransactionManager(db)

	ctx := context.Background()

	// 1. Record Not Found Test
	_, err := repo.FindByUserID(ctx, "non-existent")
	assert.ErrorIs(t, err, derror.ErrProfileNotFound)

	// 2. Save and Update Profile
	dob := time.Date(1998, 5, 20, 0, 0, 0, 0, time.UTC)
	bio, err := vo.NewBiologicalMetricsWithDOB(72.0, 175.0, dob, "MALE")
	require.NoError(t, err)

	inj, err := entity.NewInjury("inj-101", "Shoulder", "MODERATE", "Rotator cuff pain", time.Now())
	require.NoError(t, err)

	p, err := aggregate.NewUserProfile(
		"user-infra-1",
		bio,
		"INTERMEDIATE",
		[]string{"MUSCLE_GAIN"},
		[]string{"MORNING"},
		[]string{"DUMBBELL"},
		[]string{"CHEST"},
		"FRIENDLY",
		70.0,
		15.0,
		[]*entity.Injury{inj},
	)
	require.NoError(t, err)

	// Save inside transaction
	err = txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return repo.Save(txCtx, p)
	})
	require.NoError(t, err)

	// Update call
	err = repo.Update(ctx, p)
	require.NoError(t, err)

	// FindByUserID
	fetched, err := repo.FindByUserID(ctx, "user-infra-1")
	require.NoError(t, err)
	assert.Equal(t, "user-infra-1", fetched.UserID())
	assert.Equal(t, 72.0, fetched.BiologicalMetrics().WeightKg())
	assert.Equal(t, 175.0, fetched.BiologicalMetrics().HeightCm())
	assert.Len(t, fetched.Injuries(), 1)
	assert.Equal(t, "Shoulder", fetched.Injuries()[0].MuscleGroup())
	assert.False(t, fetched.Injuries()[0].IsRecovered())

	// Recover injury and update profile
	err = fetched.RecoverInjury("inj-101", time.Now())
	require.NoError(t, err)
	err = repo.Update(ctx, fetched)
	require.NoError(t, err)

	// FindByUserID must still reconstitute recovered injury
	fetchedAfterRecover, err := repo.FindByUserID(ctx, "user-infra-1")
	require.NoError(t, err)
	assert.Len(t, fetchedAfterRecover.Injuries(), 1)
	assert.True(t, fetchedAfterRecover.Injuries()[0].IsRecovered())

	// FindBodyMetricsHistory
	metricsHist, err := repo.FindBodyMetricsHistory(ctx, "user-infra-1")
	require.NoError(t, err)
	assert.NotEmpty(t, metricsHist)

	// FindInjuryHistory
	injuriesHist, err := repo.FindInjuryHistory(ctx, "user-infra-1")
	require.NoError(t, err)
	assert.Len(t, injuriesHist, 1)

	// Outbox Save & Fetch
	record := &persistence.OutboxModel{
		ID:           "out-1",
		EventID:      "evt-1",
		EventType:    "ProfileCompleted",
		Payload:      []byte(`{"user_id":"user-infra-1"}`),
		PartitionKey: "user-infra-1",
		CreatedAt:    time.Now(),
	}
	err = db.Create(record).Error
	require.NoError(t, err)

	unpub, err := outboxRepo.FetchUnpublished(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, unpub, 1)

	err = outboxRepo.MarkAsPublished(ctx, []string{"out-1"})
	require.NoError(t, err)

	unpubAfter, err := outboxRepo.FetchUnpublished(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, unpubAfter, 0)
}

func TestMapper_ToDomainAggregate_EdgeCases(t *testing.T) {
	// Test unmarshal errors and fallback in ToDomainAggregate
	userModel := &persistence.UserProfileModel{
		UserID:                "user-mapper-1",
		Gender:                "FEMALE",
		Goals:                 []byte(`invalid-json-goals`),
		PreferredWorkoutTimes: []byte(`["MORNING"]`),
		AvailableEquipment:    []byte(`["MAT"]`),
		PreferredMuscleGroups: []byte(`["LEGS"]`),
	}

	_, err := persistence.ToDomainAggregate(userModel, nil, nil)
	assert.Error(t, err)

	userModel2 := &persistence.UserProfileModel{
		UserID:                "user-mapper-2",
		Gender:                "FEMALE",
		Goals:                 []byte(`["FAT_LOSS"]`),
		PreferredWorkoutTimes: []byte(`invalid-json-times`),
	}

	_, err = persistence.ToDomainAggregate(userModel2, nil, nil)
	assert.Error(t, err)

	// Invalid periodic metric
	badMetric := &persistence.BodyMetricModel{ID: ""}
	_, err = persistence.ToDomainAggregate(userModel, []*persistence.BodyMetricModel{badMetric}, nil)
	assert.Error(t, err)
}
