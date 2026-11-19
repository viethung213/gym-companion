package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
)

// UserAllergyRepository quản lý việc lưu trữ và truy vấn thực thể dị ứng của người dùng.
type UserAllergyRepository interface {
	FindAllergiesByUserID(ctx context.Context, userID string) ([]*entity.UserAllergy, error)
	BatchFindAllergiesByUserIDs(ctx context.Context, userIDs []string) (map[string][]string, error)
	SetAllergies(ctx context.Context, userID string, allergies []*entity.UserAllergy) error
}
