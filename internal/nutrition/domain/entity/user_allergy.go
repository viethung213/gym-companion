package entity

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
)

// UserAllergy represents a user's food allergy or dietary restriction entity.
type UserAllergy struct {
	id           string
	userID       string
	allergenName string
	createdAt    time.Time
}

// NewUserAllergy creates and validates a new UserAllergy domain entity.
// It enforces invariants: valid userID, non-empty allergenName (normalized to uppercase & trimmed).
func NewUserAllergy(id, userID, allergenName string, createdAt time.Time) (*UserAllergy, error) {
	trimmedUID := strings.TrimSpace(userID)
	if trimmedUID == "" {
		return nil, domainerror.ErrUserIDRequired
	}
	normName := strings.ToUpper(strings.TrimSpace(allergenName))
	if normName == "" {
		return nil, domainerror.ErrInvalidAllergenName
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return &UserAllergy{
		id:           id,
		userID:       trimmedUID,
		allergenName: normName,
		createdAt:    createdAt,
	}, nil
}

func (a *UserAllergy) ID() string           { return a.id }
func (a *UserAllergy) UserID() string       { return a.userID }
func (a *UserAllergy) AllergenName() string { return a.allergenName }
func (a *UserAllergy) CreatedAt() time.Time { return a.createdAt }
