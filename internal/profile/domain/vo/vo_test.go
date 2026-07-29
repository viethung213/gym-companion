package vo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

func TestBiologicalMetrics(t *testing.T) {
	t.Run("Valid metrics with age", func(t *testing.T) {
		bio, err := vo.NewBiologicalMetrics(70.5, 175.0, 25, "MALE")
		require.NoError(t, err)

		assert.Equal(t, 70.5, bio.WeightKg())
		assert.Equal(t, 175.0, bio.HeightCm())
		assert.Equal(t, int32(25), bio.Age())
		assert.Equal(t, "MALE", bio.Gender())
		assert.True(t, bio.DateOfBirth().IsZero())
	})

	t.Run("Valid metrics with DateOfBirth", func(t *testing.T) {
		dob := time.Now().AddDate(-20, 0, 0)
		bio, err := vo.NewBiologicalMetricsWithDOB(65.0, 168.0, dob, "FEMALE")
		require.NoError(t, err)

		assert.Equal(t, int32(20), bio.Age())
		assert.Equal(t, dob, bio.DateOfBirth())
		assert.Equal(t, "FEMALE", bio.Gender())
	})

	t.Run("DOB later in year subtracts 1 year", func(t *testing.T) {
		// Set DOB to December 31st 20 years ago
		dob := time.Date(time.Now().Year()-20, time.December, 31, 0, 0, 0, 0, time.UTC)
		bio, err := vo.NewBiologicalMetricsWithDOB(65.0, 168.0, dob, "FEMALE")
		require.NoError(t, err)

		if time.Now().Month() != time.December || time.Now().Day() != 31 {
			assert.Equal(t, int32(19), bio.Age())
		}
	})

	t.Run("DOB in the future gives age 0", func(t *testing.T) {
		dob := time.Now().AddDate(1, 0, 0)
		bio, err := vo.NewBiologicalMetricsWithDOB(65.0, 168.0, dob, "FEMALE")
		require.NoError(t, err)
		assert.Equal(t, int32(0), bio.Age())
	})

	t.Run("Invalid biological metrics validation", func(t *testing.T) {
		_, err := vo.NewBiologicalMetrics(-5, 170, 20, "MALE")
		assert.Error(t, err)

		_, err = vo.NewBiologicalMetrics(600, 170, 20, "MALE")
		assert.Error(t, err)

		_, err = vo.NewBiologicalMetrics(70, -10, 20, "MALE")
		assert.Error(t, err)

		_, err = vo.NewBiologicalMetrics(70, 400, 20, "MALE")
		assert.Error(t, err)

		_, err = vo.NewBiologicalMetricsWithDOB(-10, 170, time.Now(), "MALE")
		assert.Error(t, err)

		_, err = vo.NewBiologicalMetricsWithDOB(70, 400, time.Now(), "MALE")
		assert.Error(t, err)
	})
}

func TestPeriodicMetric(t *testing.T) {
	t.Run("Valid periodic metric", func(t *testing.T) {
		now := time.Now()
		pm, err := vo.NewPeriodicMetric("pm-1", 72.0, 18.5, "https://photo.url/1.png", now)
		require.NoError(t, err)

		assert.Equal(t, "pm-1", pm.ID())
		assert.Equal(t, 72.0, pm.WeightKg())
		assert.Equal(t, 18.5, pm.BodyFatPercent())
		assert.Equal(t, "https://photo.url/1.png", pm.ProgressPhotoURL())
		assert.Equal(t, now, pm.LoggedAt())
	})

	t.Run("Zero loggedAt defaults to current time", func(t *testing.T) {
		pm, err := vo.NewPeriodicMetric("pm-zero", 72.0, 18.5, "", time.Time{})
		require.NoError(t, err)
		assert.False(t, pm.LoggedAt().IsZero())
	})

	t.Run("Invalid periodic metric validation", func(t *testing.T) {
		_, err := vo.NewPeriodicMetric("", 70, 15, "", time.Now())
		assert.Error(t, err)

		_, err = vo.NewPeriodicMetric("pm-2", -10, 15, "", time.Now())
		assert.Error(t, err)

		_, err = vo.NewPeriodicMetric("pm-3", 70, -5, "", time.Now())
		assert.Error(t, err)

		_, err = vo.NewPeriodicMetric("pm-4", 70, 105, "", time.Now())
		assert.Error(t, err)
	})
}
