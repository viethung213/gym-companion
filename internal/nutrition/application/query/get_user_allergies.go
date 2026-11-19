package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type GetUserAllergiesQuery struct {
	UserID string
}

type GetUserAllergiesHandler struct {
	allergyRepo repository.UserAllergyRepository
}

func NewGetUserAllergiesHandler(allergyRepo repository.UserAllergyRepository) *GetUserAllergiesHandler {
	return &GetUserAllergiesHandler{allergyRepo: allergyRepo}
}

func (h *GetUserAllergiesHandler) Handle(ctx context.Context, q GetUserAllergiesQuery) ([]string, error) {
	if strings.TrimSpace(q.UserID) == "" {
		return nil, domainerror.ErrUserIDRequired
	}
	allergies, err := h.allergyRepo.FindAllergiesByUserID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user allergies query: %w", err)
	}
	result := make([]string, 0, len(allergies))
	for _, a := range allergies {
		if a != nil {
			result = append(result, a.AllergenName())
		}
	}
	return result, nil
}
