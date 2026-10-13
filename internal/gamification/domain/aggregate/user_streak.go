package aggregate

import (
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
)

var (
	// ErrInvalidDateFormat indicates an activity date not conforming to YYYY-MM-DD.
	ErrInvalidDateFormat = errors.New("invalid activity date format, must be YYYY-MM-DD")
	// ErrFutureActivityDate indicates an activity date in the future.
	ErrFutureActivityDate = errors.New("activity date cannot be in the future")
)

// UserStreak is the aggregate root managing the user's consecutive active days.
type UserStreak struct {
	userID           string
	currentStreak    int32
	longestStreak    int32
	lastActivityDate string // YYYY-MM-DD
	updatedAt        time.Time
}

// NewUserStreak constructs a new UserStreak aggregate with 0 days.
func NewUserStreak(userID string) (*UserStreak, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	return &UserStreak{
		userID:           userID,
		currentStreak:    0,
		longestStreak:    0,
		lastActivityDate: "",
		updatedAt:        time.Now().UTC(),
	}, nil
}

// ReconstituteUserStreak reassembles a UserStreak aggregate from persistence.
func ReconstituteUserStreak(
	userID string,
	currentStreak int32,
	longestStreak int32,
	lastActivityDate string,
	updatedAt time.Time,
) *UserStreak {
	return &UserStreak{
		userID:           userID,
		currentStreak:    currentStreak,
		longestStreak:    longestStreak,
		lastActivityDate: lastActivityDate,
		updatedAt:        updatedAt,
	}
}

// UserID returns the aggregate identifier.
func (s *UserStreak) UserID() string {
	return s.userID
}

// CurrentStreak returns the current consecutive active days.
func (s *UserStreak) CurrentStreak() int32 {
	return s.currentStreak
}

// LongestStreak returns the user's lifetime record streak.
func (s *UserStreak) LongestStreak() int32 {
	return s.longestStreak
}

// LastActivityDate returns the date string (YYYY-MM-DD) of the last recorded activity.
func (s *UserStreak) LastActivityDate() string {
	return s.lastActivityDate
}

// UpdatedAt returns the timestamp when the streak was last modified.
func (s *UserStreak) UpdatedAt() time.Time {
	return s.updatedAt
}

// RecordActivity processes an activity on a specific local date and advances or resets the streak.
func (s *UserStreak) RecordActivity(
	activityDate string,
	activityType string,
	recordedAt time.Time,
) ([]event.Event, error) {
	parsedActivity, err := time.Parse("2006-01-02", activityDate)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDateFormat, err)
	}

	// Reject activity dates more than 1 day in the future (allowing +1 day for timezone transitions)
	maxAllowedDate := recordedAt.UTC().Add(24 * time.Hour)
	if parsedActivity.After(maxAllowedDate) {
		return nil, ErrFutureActivityDate
	}

	// 1. First activity ever
	if s.lastActivityDate == "" {
		s.currentStreak = 1
		s.longestStreak = 1
		s.lastActivityDate = activityDate
		s.updatedAt = recordedAt

		return []event.Event{
			event.StreakAdvanced{
				UserID:        s.userID,
				NewStreak:     1,
				LongestStreak: 1,
				ActivityDate:  activityDate,
				ActivityType:  activityType,
				AdvancedAt:    recordedAt,
			},
		}, nil
	}

	parsedLast, err := time.Parse("2006-01-02", s.lastActivityDate)
	if err != nil {
		return nil, fmt.Errorf("corrupt last_activity_date: %w", err)
	}

	daysDiff := daysBetween(parsedLast, parsedActivity)

	// 2. Same day or past activity -> Idempotent no-op
	if daysDiff <= 0 {
		return nil, nil
	}

	// 3. Consecutive day (daysDiff == 1) -> Advance streak
	if daysDiff == 1 {
		s.currentStreak++
		if s.currentStreak > s.longestStreak {
			s.longestStreak = s.currentStreak
		}
		s.lastActivityDate = activityDate
		s.updatedAt = recordedAt

		events := []event.Event{
			event.StreakAdvanced{
				UserID:        s.userID,
				NewStreak:     s.currentStreak,
				LongestStreak: s.longestStreak,
				ActivityDate:  activityDate,
				ActivityType:  activityType,
				AdvancedAt:    recordedAt,
			},
		}

		if isMilestone(s.currentStreak) {
			events = append(events, event.StreakMilestoneReached{
				UserID:        s.userID,
				MilestoneDays: s.currentStreak,
				ReachedAt:     recordedAt,
			})
		}

		return events, nil
	}

	// 4. Broken streak (daysDiff >= 2) -> Lazy reset to 1
	oldStreak := s.currentStreak
	s.currentStreak = 1
	s.lastActivityDate = activityDate
	s.updatedAt = recordedAt

	return []event.Event{
		event.StreakReset{
			UserID:         s.userID,
			PreviousStreak: oldStreak,
			ResetDate:      activityDate,
			ResetAt:        recordedAt,
		},
		event.StreakAdvanced{
			UserID:        s.userID,
			NewStreak:     1,
			LongestStreak: s.longestStreak,
			ActivityDate:  activityDate,
			ActivityType:  activityType,
			AdvancedAt:    recordedAt,
		},
	}, nil
}

func daysBetween(d1, d2 time.Time) int {
	t1 := time.Date(d1.Year(), d1.Month(), d1.Day(), 0, 0, 0, 0, time.UTC)
	t2 := time.Date(d2.Year(), d2.Month(), d2.Day(), 0, 0, 0, 0, time.UTC)
	return int(t2.Sub(t1).Hours() / 24)
}

func isMilestone(days int32) bool {
	switch days {
	case 7, 14, 30, 50, 100, 365:
		return true
	default:
		return false
	}
}
