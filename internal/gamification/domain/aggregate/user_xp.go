package aggregate

import (
	"errors"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/service"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

var (
	// ErrEmptyUserID indicates an invalid empty user ID.
	ErrEmptyUserID = errors.New("user id cannot be empty")

	// ErrInvalidXpDelta indicates a non-positive XP delta.
	ErrInvalidXpDelta = errors.New("xp delta must be greater than zero")

	// ErrNutritionRewardAlreadyClaimedToday indicates the nutrition reward has already been claimed for this date.
	ErrNutritionRewardAlreadyClaimedToday = errors.New("nutrition reward already claimed for this date")
)

// UserXp is the aggregate root managing a user's XP and Level progression.
type UserXp struct {
	userID                  string
	xp                      int64
	level                   int32
	lastNutritionRewardDate *string // YYYY-MM-DD
	updatedAt               time.Time
}

// NewUserXp constructs a fresh UserXp aggregate at level 1 with 0 XP.
func NewUserXp(userID string) (*UserXp, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	return &UserXp{
		userID:                  userID,
		xp:                      0,
		level:                   1,
		lastNutritionRewardDate: nil,
		updatedAt:               time.Now().UTC(),
	}, nil
}

// ReconstituteUserXp reassembles a UserXp aggregate from persistence data.
func ReconstituteUserXp(
	userID string,
	xp int64,
	level int32,
	lastNutritionRewardDate *string,
	updatedAt time.Time,
) *UserXp {
	var dateCopy *string
	if lastNutritionRewardDate != nil {
		d := *lastNutritionRewardDate
		dateCopy = &d
	}
	return &UserXp{
		userID:                  userID,
		xp:                      xp,
		level:                   level,
		lastNutritionRewardDate: dateCopy,
		updatedAt:               updatedAt,
	}
}

// UserID returns the aggregate identifier.
func (u *UserXp) UserID() string {
	return u.userID
}

// Xp returns the user's lifetime accumulated XP.
func (u *UserXp) Xp() int64 {
	return u.xp
}

// Level returns the user's current level [1..100].
func (u *UserXp) Level() int32 {
	return u.level
}

// LastNutritionRewardDate returns the YYYY-MM-DD date when nutrition XP was last rewarded.
func (u *UserXp) LastNutritionRewardDate() *string {
	if u.lastNutritionRewardDate == nil {
		return nil
	}
	d := *u.lastNutritionRewardDate
	return &d
}

// UpdatedAt returns the timestamp of the last XP modification.
func (u *UserXp) UpdatedAt() time.Time {
	return u.updatedAt
}

// EarnXp adds positive XP to the user's balance and evaluates potential level up.
func (u *UserXp) EarnXp(delta int32, source string, earnedAt time.Time) ([]event.Event, error) {
	if delta <= 0 {
		return nil, ErrInvalidXpDelta
	}

	u.xp += int64(delta)
	newLevel := vo.CalculateLevel(u.xp)

	events := make([]event.Event, 0, 2)
	events = append(events, &event.XpEarned{
		UserID:   u.userID,
		DeltaXp:  delta,
		NewXp:    u.xp,
		NewLevel: newLevel,
		Source:   source,
		EarnedAt: earnedAt,
	})

	if newLevel > u.level {
		events = append(events, &event.UserLeveledUp{
			UserID:      u.userID,
			OldLevel:    u.level,
			NewLevel:    newLevel,
			TotalXp:     u.xp,
			LeveledUpAt: earnedAt,
		})
		u.level = newLevel
	}

	u.updatedAt = earnedAt
	return events, nil
}

// EarnNutritionXp adds the fixed daily nutrition reward (+30 XP) if not already claimed for the given date.
func (u *UserXp) EarnNutritionXp(activityDate string, earnedAt time.Time) ([]event.Event, error) {
	if u.lastNutritionRewardDate != nil && *u.lastNutritionRewardDate == activityDate {
		return nil, ErrNutritionRewardAlreadyClaimedToday
	}

	events, err := u.EarnXp(service.NutritionDailyXpReward, "NUTRITION", earnedAt)
	if err != nil {
		return nil, err
	}

	dateCopy := activityDate
	u.lastNutritionRewardDate = &dateCopy
	return events, nil
}
