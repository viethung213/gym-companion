package persistence

import (
	"context"
	"errors"
	"fmt"

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

func (r *PostgresUserProfileRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return r.db.WithContext(ctx)
}

func (r *PostgresUserProfileRepository) Save(ctx context.Context, profile *aggregate.UserProfile) error {
	userModel, metricModels, injuryModels, err := ToPersistenceModels(profile)
	if err != nil {
		return fmt.Errorf("map domain to persistence: %w", err)
	}

	db := r.getDB(ctx)
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(userModel).Error; err != nil {
		return fmt.Errorf("save user profile model: %w", err)
	}

	for _, m := range metricModels {
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(m).Error; err != nil {
			return fmt.Errorf("save body metric model: %w", err)
		}
	}

	for _, inj := range injuryModels {
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(inj).Error; err != nil {
			return fmt.Errorf("save injury model: %w", err)
		}
	}

	return nil
}

func (r *PostgresUserProfileRepository) FindByUserID(ctx context.Context, userID string) (*aggregate.UserProfile, error) {
	db := r.getDB(ctx)
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

func (r *PostgresUserProfileRepository) FindBodyMetricsHistory(ctx context.Context, userID string) ([]vo.PeriodicMetric, error) {
	db := r.getDB(ctx)
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
	db := r.getDB(ctx)
	var injuryModels []*InjuryModel
	if err := db.Order("reported_at DESC").Find(&injuryModels, "user_id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("query injury history: %w", err)
	}

	result := make([]*entity.Injury, 0, len(injuryModels))
	for _, im := range injuryModels {
		inj := entity.ReconstituteInjury(
			im.ID,
			im.MuscleGroup,
			im.Severity,
			im.Notes,
			im.ReportedAt,
			im.IsRecovered,
			im.RecoveredAt,
		)
		result = append(result, inj)
	}
	return result, nil
}

func (r *PostgresUserProfileRepository) Update(ctx context.Context, profile *aggregate.UserProfile) error {
	return r.Save(ctx, profile)
}
