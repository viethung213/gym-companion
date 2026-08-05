package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type MealLog struct {
	id        string
	historyID string
	userID    string
	mealType  string
	mealName  string
	portion   string
	calories  float64
	protein   float64
	carbs     float64
	fat       float64
	loggedAt  time.Time
}

func NewMealLog(
	id, historyID, userID, mealType, mealName, portion string,
	calories, protein, carbs, fat float64,
	loggedAt time.Time,
) MealLog {
	return MealLog{
		id:        id,
		historyID: historyID,
		userID:    userID,
		mealType:  mealType,
		mealName:  mealName,
		portion:   portion,
		calories:  calories,
		protein:   protein,
		carbs:     carbs,
		fat:       fat,
		loggedAt:  loggedAt,
	}
}

func (m MealLog) ID() string        { return m.id }
func (m MealLog) HistoryID() string { return m.historyID }
func (m MealLog) UserID() string    { return m.userID }
func (m MealLog) MealType() string  { return m.mealType }
func (m MealLog) MealName() string  { return m.mealName }
func (m MealLog) Portion() string   { return m.portion }
func (m MealLog) Calories() float64 { return m.calories }
func (m MealLog) Protein() float64  { return m.protein }
func (m MealLog) Carbs() float64    { return m.carbs }
func (m MealLog) Fat() float64      { return m.fat }
func (m MealLog) LoggedAt() time.Time { return m.loggedAt }

// MealHistory Aggregate Root for managing historical logged meals and lockout registries.
type MealHistory struct {
	id              string
	userID          string
	mealLogs        []MealLog
	lockoutRegistry vo.LockoutRegistry
	createdAt       time.Time
	updatedAt       time.Time
}

func NewMealHistory(id, userID string, lockoutRegistry vo.LockoutRegistry) *MealHistory {
	now := time.Now()
	return &MealHistory{
		id:              id,
		userID:          userID,
		mealLogs:        make([]MealLog, 0),
		lockoutRegistry: lockoutRegistry,
		createdAt:       now,
		updatedAt:       now,
	}
}

func (h *MealHistory) ID() string                              { return h.id }
func (h *MealHistory) UserID() string                          { return h.userID }
func (h *MealHistory) LockoutRegistry() vo.LockoutRegistry { return h.lockoutRegistry }
func (h *MealHistory) MealLogs() []MealLog {
	copied := make([]MealLog, len(h.mealLogs))
	copy(copied, h.mealLogs)
	return copied
}

func (h *MealHistory) AddMealLog(log MealLog) {
	h.mealLogs = append(h.mealLogs, log)
	h.updatedAt = time.Now()
}

func (h *MealHistory) ApplyLockoutRule(itemType, itemName string, duration time.Duration, now time.Time) {
	h.lockoutRegistry = h.lockoutRegistry.ApplyLockout(itemType, itemName, duration, now)
	h.updatedAt = time.Now()
}

func (h *MealHistory) CalculateConsumedToday(targetDate time.Time) (calories, protein, carbs, fat float64) {
	targetYear, targetMonth, targetDay := targetDate.Date()
	for _, log := range h.mealLogs {
		y, m, d := log.loggedAt.Date()
		if y == targetYear && m == targetMonth && d == targetDay {
			calories += log.calories
			protein += log.protein
			carbs += log.carbs
			fat += log.fat
		}
	}
	return calories, protein, carbs, fat
}
