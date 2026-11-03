package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ repository.SettingRepository = (*SettingRepository)(nil)

type SettingRepository struct {
	db *gorm.DB
}

func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

func (r *SettingRepository) GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error) {
	var m SettingModel
	err := GetDB(ctx, r.db).Where("user_id = ?", userID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrSettingNotFound
		}
		return nil, fmt.Errorf("query user notification setting: %w", err)
	}

	return m.ToDomain(), nil
}

func (r *SettingRepository) Save(ctx context.Context, setting *aggregate.Setting) error {
	m := ToSettingModel(setting)
	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enable_push",
			"enable_email",
			"enable_sms",
			"quiet_hours_start",
			"quiet_hours_end",
			"updated_at",
		}),
	}).Create(m).Error
	if err != nil {
		return fmt.Errorf("upsert user notification setting: %w", err)
	}
	return nil
}
