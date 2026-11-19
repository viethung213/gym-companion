package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type UpdateUserAllergiesCommand struct {
	UserID    string
	Allergies []string
}

type UpdateUserAllergiesHandler struct {
	allergyRepo repository.UserAllergyRepository
}

func NewUpdateUserAllergiesHandler(allergyRepo repository.UserAllergyRepository) *UpdateUserAllergiesHandler {
	return &UpdateUserAllergiesHandler{allergyRepo: allergyRepo}
}

func (h *UpdateUserAllergiesHandler) Handle(ctx context.Context, cmd UpdateUserAllergiesCommand) ([]string, error) {
	if strings.TrimSpace(cmd.UserID) == "" {
		return nil, domainerror.ErrUserIDRequired
	}

	seen := make(map[string]bool, len(cmd.Allergies))
	entities := make([]*entity.UserAllergy, 0, len(cmd.Allergies))
	cleanNames := make([]string, 0, len(cmd.Allergies))
	now := time.Now()

	for _, a := range cmd.Allergies {
		norm := strings.ToUpper(strings.TrimSpace(a))
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true

		allergyEntity, err := entity.NewUserAllergy(uuid.NewString(), cmd.UserID, norm, now)
		if err != nil {
			return nil, err
		}
		entities = append(entities, allergyEntity)
		cleanNames = append(cleanNames, allergyEntity.AllergenName())
	}

	if err := h.allergyRepo.SetAllergies(ctx, cmd.UserID, entities); err != nil {
		return nil, fmt.Errorf("update user allergies: %w", err)
	}

	return cleanNames, nil
}
