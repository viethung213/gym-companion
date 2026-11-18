package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
)

func TestNewWeeklyInsight_Success(t *testing.T) {
	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()

	insight, err := entity.NewWeeklyInsight(
		"insight-1",
		"user-1",
		start,
		end,
		85,
		"Ăn uống tốt",
		"Luộc / Hấp",
		[]string{"Ức gà", "Bông cải xanh"},
		[]entity.WeeklyImprovementArea{
			{Nutrient: "Fat", Action: "GIAM", DeltaAmount: 10, Suggestion: "Giảm dầu mỡ"},
		},
		&entity.WeeklyMacroSummary{
			ConsumedCalories: 2000,
			TargetCalories:   2100,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, insight)

	assert.Equal(t, "insight-1", insight.ID())
	assert.Equal(t, "user-1", insight.UserID())
	assert.Equal(t, start, insight.StartDate())
	assert.Equal(t, end, insight.EndDate())
	assert.Equal(t, 85, insight.OverallScore())
	assert.Equal(t, "Ăn uống tốt", insight.Summary())
	assert.Equal(t, "Luộc / Hấp", insight.PredictedCookingStyle())
	assert.Equal(t, []string{"Ức gà", "Bông cải xanh"}, insight.PreferredIngredients())
	assert.Len(t, insight.ImprovementAreas(), 1)
	assert.Equal(t, 2000.0, insight.WeeklySummary().ConsumedCalories)
	assert.False(t, insight.CreatedAt().IsZero())
	assert.False(t, insight.UpdatedAt().IsZero())
}

func TestNewWeeklyInsight_InvariantsValidation(t *testing.T) {
	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()

	t.Run("empty id", func(t *testing.T) {
		_, err := entity.NewWeeklyInsight(
			"", "user-1", start, end, 80, "summary", "style", nil, nil, nil,
		)
		assert.ErrorIs(t, err, entity.ErrEmptyWeeklyInsightID)
	})

	t.Run("empty user id", func(t *testing.T) {
		_, err := entity.NewWeeklyInsight(
			"insight-1", "", start, end, 80, "summary", "style", nil, nil, nil,
		)
		assert.ErrorIs(t, err, entity.ErrEmptyWeeklyInsightUserID)
	})

	t.Run("start date after end date", func(t *testing.T) {
		_, err := entity.NewWeeklyInsight(
			"insight-1", "user-1", end, start, 80, "summary", "style", nil, nil, nil,
		)
		assert.ErrorIs(t, err, entity.ErrInvalidWeeklyInsightDateRange)
	})

	t.Run("score below 0", func(t *testing.T) {
		_, err := entity.NewWeeklyInsight(
			"insight-1", "user-1", start, end, -5, "summary", "style", nil, nil, nil,
		)
		assert.ErrorIs(t, err, entity.ErrInvalidWeeklyInsightScore)
	})

	t.Run("score above 100", func(t *testing.T) {
		_, err := entity.NewWeeklyInsight(
			"insight-1", "user-1", start, end, 105, "summary", "style", nil, nil, nil,
		)
		assert.ErrorIs(t, err, entity.ErrInvalidWeeklyInsightScore)
	})
}

func TestReconstructWeeklyInsight_Success(t *testing.T) {
	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()
	created := start.Add(time.Hour)
	updated := end.Add(-time.Hour)

	insight, err := entity.ReconstructWeeklyInsight(
		"insight-db-1",
		"user-1",
		start,
		end,
		90,
		"Duy trì xuất sắc",
		"Xào ít dầu",
		[]string{"Thịt bò"},
		nil,
		nil,
		created,
		updated,
	)
	require.NoError(t, err)
	require.NotNil(t, insight)
	assert.Equal(t, created, insight.CreatedAt())
	assert.Equal(t, updated, insight.UpdatedAt())
}

func TestReconstructWeeklyInsight_InvariantsValidation(t *testing.T) {
	start := time.Now().AddDate(0, 0, -7)
	end := time.Now()

	_, err := entity.ReconstructWeeklyInsight(
		"", "user-1", start, end, 90, "summary", "style", nil, nil,
		nil, time.Now(), time.Now(),
	)
	assert.ErrorIs(t, err, entity.ErrEmptyWeeklyInsightID)
}
