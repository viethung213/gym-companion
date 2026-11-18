package entity

import (
	"errors"
	"time"
)

var (
	ErrEmptyWeeklyInsightID          = errors.New("weekly insight id cannot be empty")
	ErrEmptyWeeklyInsightUserID      = errors.New("weekly insight user id cannot be empty")
	ErrInvalidWeeklyInsightDateRange = errors.New("weekly insight start date must be before end date")
	ErrInvalidWeeklyInsightScore     = errors.New("weekly insight overall score must be between 0 and 100")
)

type WeeklyImprovementArea struct {
	Nutrient    string
	Action      string
	DeltaAmount float64
	Suggestion  string
}

type WeeklyMacroSummary struct {
	ConsumedCalories float64
	TargetCalories   float64
	ConsumedProtein  float64
	TargetProtein    float64
	ConsumedCarbs    float64
	TargetCarbs      float64
	ConsumedFat      float64
	TargetFat        float64
	LoggedMealsCount int
	TotalMealsCount  int
}

type WeeklyInsight struct {
	id                    string
	userID                string
	startDate             time.Time
	endDate               time.Time
	overallScore          int
	summary               string
	predictedCookingStyle string
	preferredIngredients  []string
	improvementAreas      []WeeklyImprovementArea
	weeklySummary         WeeklyMacroSummary
	createdAt             time.Time
	updatedAt             time.Time
}

// NewWeeklyInsight tạo mới một thực thể WeeklyInsight với đầy đủ ràng buộc nghiệp vụ (invariants).
func NewWeeklyInsight(
	id string,
	userID string,
	startDate time.Time,
	endDate time.Time,
	overallScore int,
	summary string,
	predictedCookingStyle string,
	preferredIngredients []string,
	improvementAreas []WeeklyImprovementArea,
	weeklySummary *WeeklyMacroSummary,
) (*WeeklyInsight, error) {
	if err := validateInvariants(id, userID, startDate, endDate, overallScore); err != nil {
		return nil, err
	}

	prefCopy := make([]string, len(preferredIngredients))
	copy(prefCopy, preferredIngredients)

	areasCopy := make([]WeeklyImprovementArea, len(improvementAreas))
	copy(areasCopy, improvementAreas)

	var summaryVal WeeklyMacroSummary
	if weeklySummary != nil {
		summaryVal = *weeklySummary
	}

	now := time.Now()

	return &WeeklyInsight{
		id:                    id,
		userID:                userID,
		startDate:             startDate,
		endDate:               endDate,
		overallScore:          overallScore,
		summary:               summary,
		predictedCookingStyle: predictedCookingStyle,
		preferredIngredients:  prefCopy,
		improvementAreas:      areasCopy,
		weeklySummary:         summaryVal,
		createdAt:             now,
		updatedAt:             now,
	}, nil
}

// ReconstructWeeklyInsight tái tạo thực thể từ tầng lưu trữ (persistence/database).
func ReconstructWeeklyInsight(
	id string,
	userID string,
	startDate time.Time,
	endDate time.Time,
	overallScore int,
	summary string,
	predictedCookingStyle string,
	preferredIngredients []string,
	improvementAreas []WeeklyImprovementArea,
	weeklySummary *WeeklyMacroSummary,
	createdAt time.Time,
	updatedAt time.Time,
) (*WeeklyInsight, error) {
	if err := validateInvariants(id, userID, startDate, endDate, overallScore); err != nil {
		return nil, err
	}

	prefCopy := make([]string, len(preferredIngredients))
	copy(prefCopy, preferredIngredients)

	areasCopy := make([]WeeklyImprovementArea, len(improvementAreas))
	copy(areasCopy, improvementAreas)

	var summaryVal WeeklyMacroSummary
	if weeklySummary != nil {
		summaryVal = *weeklySummary
	}

	return &WeeklyInsight{
		id:                    id,
		userID:                userID,
		startDate:             startDate,
		endDate:               endDate,
		overallScore:          overallScore,
		summary:               summary,
		predictedCookingStyle: predictedCookingStyle,
		preferredIngredients:  prefCopy,
		improvementAreas:      areasCopy,
		weeklySummary:         summaryVal,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
	}, nil
}

func validateInvariants(id, userID string, startDate, endDate time.Time, overallScore int) error {
	if id == "" {
		return ErrEmptyWeeklyInsightID
	}
	if userID == "" {
		return ErrEmptyWeeklyInsightUserID
	}
	if !startDate.Before(endDate) {
		return ErrInvalidWeeklyInsightDateRange
	}
	if overallScore < 0 || overallScore > 100 {
		return ErrInvalidWeeklyInsightScore
	}
	return nil
}

func (w *WeeklyInsight) ID() string                    { return w.id }
func (w *WeeklyInsight) UserID() string                { return w.userID }
func (w *WeeklyInsight) StartDate() time.Time          { return w.startDate }
func (w *WeeklyInsight) EndDate() time.Time            { return w.endDate }
func (w *WeeklyInsight) OverallScore() int             { return w.overallScore }
func (w *WeeklyInsight) Summary() string               { return w.summary }
func (w *WeeklyInsight) PredictedCookingStyle() string { return w.predictedCookingStyle }
func (w *WeeklyInsight) PreferredIngredients() []string {
	copied := make([]string, len(w.preferredIngredients))
	copy(copied, w.preferredIngredients)
	return copied
}
func (w *WeeklyInsight) ImprovementAreas() []WeeklyImprovementArea {
	copied := make([]WeeklyImprovementArea, len(w.improvementAreas))
	copy(copied, w.improvementAreas)
	return copied
}
func (w *WeeklyInsight) WeeklySummary() WeeklyMacroSummary { return w.weeklySummary }
func (w *WeeklyInsight) CreatedAt() time.Time              { return w.createdAt }
func (w *WeeklyInsight) UpdatedAt() time.Time              { return w.updatedAt }
