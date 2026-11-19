package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
)

func TestNewUserAllergy(t *testing.T) {
	now := time.Now()

	t.Run("valid creation with uppercase and trim", func(t *testing.T) {
		a, err := entity.NewUserAllergy("allergy-1", "user-1", "  peanut  ", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.ID() != "allergy-1" {
			t.Errorf("got id %v, want allergy-1", a.ID())
		}
		if a.UserID() != "user-1" {
			t.Errorf("got userID %v, want user-1", a.UserID())
		}
		if a.AllergenName() != "PEANUT" {
			t.Errorf("got allergenName %v, want PEANUT", a.AllergenName())
		}
		if !a.CreatedAt().Equal(now) {
			t.Errorf("got createdAt %v, want %v", a.CreatedAt(), now)
		}
	})

	t.Run("empty user id returns ErrUserIDRequired", func(t *testing.T) {
		_, err := entity.NewUserAllergy("allergy-1", "   ", "peanut", now)
		if !errors.Is(err, domainerror.ErrUserIDRequired) {
			t.Errorf("got %v, want ErrUserIDRequired", err)
		}
	})

	t.Run("empty allergen name returns ErrInvalidAllergenName", func(t *testing.T) {
		_, err := entity.NewUserAllergy("allergy-1", "user-1", "   ", now)
		if !errors.Is(err, domainerror.ErrInvalidAllergenName) {
			t.Errorf("got %v, want ErrInvalidAllergenName", err)
		}
	})
}
