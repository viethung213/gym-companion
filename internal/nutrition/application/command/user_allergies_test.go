package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/application/query"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
)

type mockUserAllergyRepo struct {
	allergies map[string][]*entity.UserAllergy
}

func newMockUserAllergyRepo() *mockUserAllergyRepo {
	return &mockUserAllergyRepo{allergies: make(map[string][]*entity.UserAllergy)}
}

func (m *mockUserAllergyRepo) FindAllergiesByUserID(_ context.Context, userID string) ([]*entity.UserAllergy, error) {
	return m.allergies[userID], nil
}

func (m *mockUserAllergyRepo) BatchFindAllergiesByUserIDs(_ context.Context, userIDs []string) (map[string][]string, error) {
	res := make(map[string][]string, len(userIDs))
	for _, uid := range userIDs {
		names := make([]string, 0, len(m.allergies[uid]))
		for _, a := range m.allergies[uid] {
			names = append(names, a.AllergenName())
		}
		res[uid] = names
	}
	return res, nil
}

func (m *mockUserAllergyRepo) SetAllergies(_ context.Context, userID string, allergies []*entity.UserAllergy) error {
	m.allergies[userID] = allergies
	return nil
}

func TestUserAllergies_NormalizationAndDeduplication(t *testing.T) {
	ctx := context.Background()
	repo := newMockUserAllergyRepo()

	updateHdlr := command.NewUpdateUserAllergiesHandler(repo)
	getHdlr := query.NewGetUserAllergiesHandler(repo)

	userID := "user-allergy-test-1"

	// 1. Validation: empty user ID
	_, err := updateHdlr.Handle(ctx, command.UpdateUserAllergiesCommand{UserID: ""})
	if !errors.Is(err, domainerror.ErrUserIDRequired) {
		t.Fatalf("expected ErrUserIDRequired, got %v", err)
	}

	_, err = getHdlr.Handle(ctx, query.GetUserAllergiesQuery{UserID: ""})
	if !errors.Is(err, domainerror.ErrUserIDRequired) {
		t.Fatalf("expected ErrUserIDRequired, got %v", err)
	}

	// 2. UpdateUserAllergies: Normalization & Deduplication
	// Input: ["  peanut  ", "PEANUT", "Peanut", "  ", "shrimp  "]
	res, err := updateHdlr.Handle(ctx, command.UpdateUserAllergiesCommand{
		UserID: userID,
		Allergies: []string{
			"  peanut  ",
			"PEANUT",
			"Peanut",
			"  ",
			"shrimp  ",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on UpdateUserAllergies: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected exactly 2 deduplicated allergies, got %d: %v", len(res), res)
	}
	if res[0] != "PEANUT" || res[1] != "SHRIMP" {
		t.Fatalf("expected [PEANUT SHRIMP], got %v", res)
	}

	// Verify query returns same list
	got, err := getHdlr.Handle(ctx, query.GetUserAllergiesQuery{UserID: userID})
	if err != nil {
		t.Fatalf("unexpected error on GetUserAllergies: %v", err)
	}
	if len(got) != 2 || got[0] != "PEANUT" || got[1] != "SHRIMP" {
		t.Fatalf("expected [PEANUT SHRIMP], got %v", got)
	}

	// 3. UpdateUserAllergies: overwrite with updated list
	res2, err := updateHdlr.Handle(ctx, command.UpdateUserAllergiesCommand{
		UserID:    userID,
		Allergies: []string{"dairy", "fish"},
	})
	if err != nil {
		t.Fatalf("unexpected error on UpdateUserAllergies: %v", err)
	}
	if len(res2) != 2 || res2[0] != "DAIRY" || res2[1] != "FISH" {
		t.Fatalf("expected [DAIRY FISH], got %v", res2)
	}
}
