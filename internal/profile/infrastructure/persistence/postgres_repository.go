package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/repository"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresUserProfileRepository struct {
	db *gorm.DB
}

var _ repository.UserProfileRepository = (*PostgresUserProfileRepository)(nil)

func NewPostgresUserProfileRepository(db *gorm.DB) *PostgresUserProfileRepository {
	return &PostgresUserProfileRepository{db: db}
}

func (r *PostgresUserProfileRepository) Save(ctx context.Context, profile *aggregate.UserProfile) error {
	userModel, metricModels, injuryModels, err := ToPersistenceModels(profile)
	if err != nil {
		return fmt.Errorf("map domain to persistence: %w", err)
	}

	db := GetDB(ctx, r.db)
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(userModel).Error; err != nil {
		return fmt.Errorf("save user profile model: %w", err)
	}

	if len(metricModels) > 0 {
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(metricModels, 100).Error; err != nil {
			return fmt.Errorf("save body metric models batch: %w", err)
		}
	}

	if len(injuryModels) > 0 {
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(injuryModels, 100).Error; err != nil {
			return fmt.Errorf("save injury models batch: %w", err)
		}
	}

	return nil
}

func (r *PostgresUserProfileRepository) FindByUserID(ctx context.Context, userID string) (*aggregate.UserProfile, error) {
	db := GetDB(ctx, r.db)
	var userModel UserProfileModel
	if err := db.First(&userModel, "user_id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, derror.ErrProfileNotFound
		}
		return nil, fmt.Errorf("query user profile: %w", err)
	}

	// Only query latest body metric for active profile state
	var metricModels []*BodyMetricModel
	if err := db.Order("logged_at DESC").Limit(1).Find(&metricModels, "user_id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("query body metrics: %w", err)
	}

	var injuryModels []*InjuryModel
	if err := db.Find(&injuryModels, "user_id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("query injuries: %w", err)
	}

	domain, err := ToDomainAggregate(&userModel, metricModels, injuryModels)
	if err != nil {
		return nil, fmt.Errorf("map DB models to domain aggregate: %w", err)
	}

	return domain, nil
}

func (r *PostgresUserProfileRepository) UpdateProfile(ctx context.Context, profile *aggregate.UserProfile) error {
	userModel, _, _, err := ToPersistenceModels(profile)
	if err != nil {
		return fmt.Errorf("map domain to persistence: %w", err)
	}

	db := GetDB(ctx, r.db)
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(userModel).Error; err != nil {
		return fmt.Errorf("update user profile: %w", err)
	}
	return nil
}

func (r *PostgresUserProfileRepository) UpdateIdentity(ctx context.Context, userID, fullName, avatarURL string) error {
	db := GetDB(ctx, r.db)
	now := time.Now()
	updates := map[string]any{
		"full_name":  fullName,
		"avatar_url": avatarURL,
		"updated_at": now,
	}

	res := db.Model(&UserProfileModel{}).Where("user_id = ?", userID).Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("update identity in db: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return derror.ErrProfileNotFound
	}
	return nil
}

//nolint:gocritic // heavy value object passed by value
func (r *PostgresUserProfileRepository) AddPeriodicMetric(
	ctx context.Context,
	userID string,
	metric vo.PeriodicMetric,
	updatedBio vo.BiologicalMetrics,
) error {
	db := GetDB(ctx, r.db)

	h := metric.HeightCm()
	if h <= 0 {
		h = updatedBio.HeightCm()
	}

	metricModel := &BodyMetricModel{
		ID:               metric.ID(),
		UserID:           userID,
		WeightKg:         metric.WeightKg(),
		HeightCm:         h,
		BodyFatPercent:   metric.BodyFatPercent(),
		ProgressPhotoURL: metric.ProgressPhotoURL(),
		LoggedAt:         metric.LoggedAt(),
	}

	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(metricModel).Error; err != nil {
		return fmt.Errorf("insert periodic metric: %w", err)
	}

	now := time.Now()
	if err := db.Model(&UserProfileModel{}).Where("user_id = ?", userID).Update("updated_at", now).Error; err != nil {
		return fmt.Errorf("update user profile timestamp on periodic metric: %w", err)
	}

	return nil
}

func (r *PostgresUserProfileRepository) AddInjury(ctx context.Context, userID string, injury *entity.Injury) error {
	db := GetDB(ctx, r.db)
	now := time.Now()
	injuryModel := newInjuryModel(userID, injury, now)

	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(injuryModel).Error; err != nil {
		return fmt.Errorf("insert injury: %w", err)
	}

	if err := db.Model(&UserProfileModel{}).Where("user_id = ?", userID).Update("updated_at", now).Error; err != nil {
		return fmt.Errorf("update user profile timestamp on injury: %w", err)
	}

	return nil
}

func (r *PostgresUserProfileRepository) RecoverInjury(ctx context.Context, userID, injuryID string) error {
	db := GetDB(ctx, r.db)
	now := time.Now()

	res := db.Model(&InjuryModel{}).
		Where("id = ? AND user_id = ?", injuryID, userID).
		Updates(map[string]any{
			"is_recovered": true,
			"recovered_at": &now,
			"updated_at":   now,
		})
	if res.Error != nil {
		return fmt.Errorf("recover injury: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return derror.ErrInjuryNotFound
	}

	if err := db.Model(&UserProfileModel{}).Where("user_id = ?", userID).Update("updated_at", now).Error; err != nil {
		return fmt.Errorf("update user profile timestamp on recover injury: %w", err)
	}

	return nil
}

func (r *PostgresUserProfileRepository) FindBodyMetricsHistory(ctx context.Context, userID string) ([]vo.PeriodicMetric, error) {
	db := GetDB(ctx, r.db)
	var metricModels []*BodyMetricModel
	if err := db.Order("logged_at DESC").Find(&metricModels, "user_id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("query body metrics history: %w", err)
	}

	result := make([]vo.PeriodicMetric, 0, len(metricModels))
	for _, mm := range metricModels {
		pm, err := vo.NewPeriodicMetric(mm.ID, mm.WeightKg, mm.BodyFatPercent, mm.ProgressPhotoURL, mm.LoggedAt, mm.HeightCm)
		if err != nil {
			continue
		}
		result = append(result, pm)
	}
	return result, nil
}

func (r *PostgresUserProfileRepository) FindInjuryHistory(ctx context.Context, userID string) ([]*entity.Injury, error) {
	db := GetDB(ctx, r.db)
	var injuryModels []*InjuryModel
	if err := db.Order("reported_at DESC").Find(&injuryModels, "user_id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("query injury history: %w", err)
	}

	result := make([]*entity.Injury, 0, len(injuryModels))
	for _, im := range injuryModels {
		inj, err := im.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, inj)
	}
	return result, nil
}
