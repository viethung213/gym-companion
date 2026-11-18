package entity

import "time"

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

// NewWeeklyInsight khởi tạo một WeeklyInsight entity hoàn chỉnh.
//
//nolint:gocritic // value object is copied into entity constructor
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
	weeklySummary WeeklyMacroSummary,
	createdAt time.Time,
	updatedAt time.Time,
) *WeeklyInsight {
	prefCopy := make([]string, len(preferredIngredients))
	copy(prefCopy, preferredIngredients)

	areasCopy := make([]WeeklyImprovementArea, len(improvementAreas))
	copy(areasCopy, improvementAreas)

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
		weeklySummary:         weeklySummary,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
	}
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
