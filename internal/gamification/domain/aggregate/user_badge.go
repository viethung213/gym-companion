package aggregate

import (
	"errors"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
)

var (
	// ErrEmptyBadgeCode indicates an empty badge code string.
	ErrEmptyBadgeCode = errors.New("badge code cannot be empty")
)

// UserBadge represents an unlocked achievement honor for a user.
type UserBadge struct {
	id         string
	userID     string
	badgeCode  string
	badgeName  string
	category   string
	unlockedAt time.Time
	metadata   map[string]string
}

// UnlockBadge constructs a new UserBadge and produces a BadgeUnlocked domain event.
func UnlockBadge(
	id string,
	userID string,
	badgeCode string,
	badgeName string,
	category string,
	metadata map[string]string,
	unlockedAt time.Time,
) (*UserBadge, event.Event, error) {
	if userID == "" {
		return nil, nil, ErrEmptyUserID
	}
	if badgeCode == "" {
		return nil, nil, ErrEmptyBadgeCode
	}

	metaCopy := make(map[string]string, len(metadata))
	for k, v := range metadata {
		metaCopy[k] = v
	}

	badge := &UserBadge{
		id:         id,
		userID:     userID,
		badgeCode:  badgeCode,
		badgeName:  badgeName,
		category:   category,
		unlockedAt: unlockedAt,
		metadata:   metaCopy,
	}

	ev := &event.BadgeUnlocked{
		UserID:     userID,
		BadgeCode:  badgeCode,
		BadgeName:  badgeName,
		Category:   category,
		UnlockedAt: unlockedAt,
	}

	return badge, ev, nil
}

// ReconstituteUserBadge reassembles a UserBadge from persistence data.
func ReconstituteUserBadge(
	id string,
	userID string,
	badgeCode string,
	badgeName string,
	category string,
	unlockedAt time.Time,
	metadata map[string]string,
) *UserBadge {
	metaCopy := make(map[string]string, len(metadata))
	for k, v := range metadata {
		metaCopy[k] = v
	}
	return &UserBadge{
		id:         id,
		userID:     userID,
		badgeCode:  badgeCode,
		badgeName:  badgeName,
		category:   category,
		unlockedAt: unlockedAt,
		metadata:   metaCopy,
	}
}

// ID returns the badge record identifier.
func (b *UserBadge) ID() string {
	return b.id
}

// UserID returns the owning user's identifier.
func (b *UserBadge) UserID() string {
	return b.userID
}

// BadgeCode returns the programmatic unique badge code.
func (b *UserBadge) BadgeCode() string {
	return b.badgeCode
}

// BadgeName returns the human-readable badge name.
func (b *UserBadge) BadgeName() string {
	return b.badgeName
}

// Category returns the badge classification category.
func (b *UserBadge) Category() string {
	return b.category
}

// UnlockedAt returns the time the badge was attained.
func (b *UserBadge) UnlockedAt() time.Time {
	return b.unlockedAt
}

// Metadata returns a defensive copy of the badge metadata.
func (b *UserBadge) Metadata() map[string]string {
	copyMap := make(map[string]string, len(b.metadata))
	for k, v := range b.metadata {
		copyMap[k] = v
	}
	return copyMap
}
