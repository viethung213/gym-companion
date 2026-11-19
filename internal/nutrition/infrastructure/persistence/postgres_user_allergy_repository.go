package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/entity"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"gorm.io/gorm"
)

type GormUserAllergy struct {
	ID           string    `gorm:"column:id;type:uuid;primaryKey"`
	UserID       string    `gorm:"column:user_id;type:uuid;not null;index:idx_nutrition_user_allergies_user_id"`
	AllergenName string    `gorm:"column:allergen_name;type:varchar(100);not null"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (GormUserAllergy) TableName() string {
	return "nutrition.user_allergies"
}

type PostgresUserAllergyRepository struct {
	db *gorm.DB
}

var _ repository.UserAllergyRepository = (*PostgresUserAllergyRepository)(nil)

func NewPostgresUserAllergyRepository(db *gorm.DB) *PostgresUserAllergyRepository {
	return &PostgresUserAllergyRepository{db: db}
}

func (r *PostgresUserAllergyRepository) FindAllergiesByUserID(ctx context.Context, userID string) ([]*entity.UserAllergy, error) {
	trimmedUID := strings.TrimSpace(userID)
	if trimmedUID == "" {
		return nil, domainerror.ErrUserIDRequired
	}
	db := GetDB(ctx, r.db)
	var records []GormUserAllergy
	err := db.WithContext(ctx).
		Where("user_id = ?", trimmedUID).
		Order("allergen_name ASC").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("postgres user allergy repo find by user id: %w", err)
	}
	result := make([]*entity.UserAllergy, 0, len(records))
	for i := range records {
		result = append(result, records[i].ToDomain())
	}
	return result, nil
}

func (r *PostgresUserAllergyRepository) BatchFindAllergiesByUserIDs(ctx context.Context, userIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	validIDs := make([]string, 0, len(userIDs))
	for _, uid := range userIDs {
		trimmed := strings.TrimSpace(uid)
		if trimmed != "" {
			validIDs = append(validIDs, trimmed)
			result[trimmed] = make([]string, 0)
		}
	}
	if len(validIDs) == 0 {
		return result, nil
	}

	db := GetDB(ctx, r.db)
	var records []GormUserAllergy
	err := db.WithContext(ctx).
		Where("user_id IN ?", validIDs).
		Order("user_id, allergen_name ASC").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("postgres user allergy repo batch find: %w", err)
	}
	for _, rec := range records {
		result[rec.UserID] = append(result[rec.UserID], rec.AllergenName)
	}
	return result, nil
}

func (r *PostgresUserAllergyRepository) SetAllergies(ctx context.Context, userID string, allergies []*entity.UserAllergy) error {
	trimmedUID := strings.TrimSpace(userID)
	if trimmedUID == "" {
		return domainerror.ErrUserIDRequired
	}
	db := GetDB(ctx, r.db)

	newEntityMap := make(map[string]*entity.UserAllergy, len(allergies))
	for _, a := range allergies {
		if a != nil && a.AllergenName() != "" {
			newEntityMap[a.AllergenName()] = a
		}
	}

	// 1. Fetch current allergies from DB
	var currentRecords []GormUserAllergy
	if err := db.WithContext(ctx).Where("user_id = ?", trimmedUID).Find(&currentRecords).Error; err != nil {
		return fmt.Errorf("fetch existing allergies: %w", err)
	}

	currentSet := make(map[string]bool, len(currentRecords))
	for _, rec := range currentRecords {
		currentSet[rec.AllergenName] = true
	}

	// 2. Identify items to delete and items to insert
	var toDelete []string
	for _, rec := range currentRecords {
		if _, exists := newEntityMap[rec.AllergenName]; !exists {
			toDelete = append(toDelete, rec.AllergenName)
		}
	}

	var toInsert []*entity.UserAllergy
	for name, item := range newEntityMap {
		if !currentSet[name] {
			toInsert = append(toInsert, item)
		}
	}

	// 3. Early exit if nothing changed (idempotent, 0 write I/O)
	if len(toDelete) == 0 && len(toInsert) == 0 {
		return nil
	}

	// 4. Execute changes in transaction using Domain mapper
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(toDelete) > 0 {
			if err := tx.Where("user_id = ? AND allergen_name IN ?", trimmedUID, toDelete).
				Delete(&GormUserAllergy{}).Error; err != nil {
				return fmt.Errorf("delete obsolete user allergies: %w", err)
			}
		}
		if len(toInsert) > 0 {
			records := make([]GormUserAllergy, 0, len(toInsert))
			for _, item := range toInsert {
				records = append(records, FromDomainUserAllergy(item))
			}
			if err := tx.Create(&records).Error; err != nil {
				return fmt.Errorf("insert new user allergies: %w", err)
			}
		}
		return nil
	})
}
