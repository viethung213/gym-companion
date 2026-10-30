package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormBadgeRepository implements repository.BadgeRepository.
type GormBadgeRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ repository.BadgeRepository = (*GormBadgeRepository)(nil)

// NewGormBadgeRepository constructs a new GormBadgeRepository.
func NewGormBadgeRepository(db *gorm.DB) *GormBadgeRepository {
	return &GormBadgeRepository{db: db}
}

// FindByUserID retrieves all badges unlocked by the given user.
func (r *GormBadgeRepository) FindByUserID(ctx context.Context, userID string) ([]*aggregate.UserBadge, error) {
	db := GetDB(ctx, r.db)
	var models []*UserBadgeModel
	err := db.Where("user_id = ?", userID).
		Order("unlocked_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("find badges by user id: %w", err)
	}

	badges := make([]*aggregate.UserBadge, 0, len(models))
	for _, m := range models {
		var meta map[string]string
		if len(m.Metadata) > 0 {
			_ = json.Unmarshal(m.Metadata, &meta)
		}
		def, _ := vo.GetBadgeDefinition(m.BadgeCode)
		badges = append(badges, aggregate.ReconstituteUserBadge(
			m.ID,
			m.UserID,
			m.BadgeCode,
			def.Name,
			def.Category,
			m.UnlockedAt,
			meta,
		))
	}

	return badges, nil
}

// HasBadge checks if a user has already unlocked a specific badge code.
func (r *GormBadgeRepository) HasBadge(ctx context.Context, userID, badgeCode string) (bool, error) {
	db := GetDB(ctx, r.db)
	var count int64
	err := db.Model(&UserBadgeModel{}).
		Where("user_id = ? AND badge_code = ?", userID, badgeCode).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check has badge: %w", err)
	}
	return count > 0, nil
}

// Save stores a new UserBadge entry into user_badges with conflict suppression.
func (r *GormBadgeRepository) Save(ctx context.Context, badge *aggregate.UserBadge) error {
	if badge == nil {
		return errors.New("nil user badge")
	}

	id := badge.ID()
	if id == "" {
		id = uuid.NewString()
	}

	metaBytes, err := json.Marshal(badge.Metadata())
	if err != nil {
		metaBytes = []byte("{}")
	}

	model := &UserBadgeModel{
		ID:         id,
		UserID:     badge.UserID(),
		BadgeCode:  badge.BadgeCode(),
		UnlockedAt: badge.UnlockedAt(),
		Metadata:   metaBytes,
	}

	db := GetDB(ctx, r.db)
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "badge_code"}},
		DoNothing: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save user badge: %w", err)
	}

	return nil
}

// SaveBatch persists multiple UserBadge entries into user_badges in a single atomic batch insert.
func (r *GormBadgeRepository) SaveBatch(ctx context.Context, badges []*aggregate.UserBadge) error {
	_, err := r.SaveBatchReturningInserted(ctx, badges)
	return err
}

// SaveBatchReturningInserted persists multiple UserBadge entries and returns the badge codes that were newly inserted.
func (r *GormBadgeRepository) SaveBatchReturningInserted(ctx context.Context, badges []*aggregate.UserBadge) ([]string, error) {
	if len(badges) == 0 {
		return nil, nil
	}

	models := make([]*UserBadgeModel, 0, len(badges))
	for _, badge := range badges {
		if badge == nil {
			continue
		}
		id := badge.ID()
		if id == "" {
			id = uuid.NewString()
		}

		metaBytes, err := json.Marshal(badge.Metadata())
		if err != nil {
			metaBytes = []byte("{}")
		}

		models = append(models, &UserBadgeModel{
			ID:         id,
			UserID:     badge.UserID(),
			BadgeCode:  badge.BadgeCode(),
			UnlockedAt: badge.UnlockedAt(),
			Metadata:   metaBytes,
		})
	}

	if len(models) == 0 {
		return nil, nil
	}

	db := GetDB(ctx, r.db)
	if len(models) == 1 {
		res := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "badge_code"}},
			DoNothing: true,
		}).Create(models[0])
		if res.Error != nil {
			return nil, fmt.Errorf("batch save user badges: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil, nil
		}
		return []string{models[0].BadgeCode}, nil
	}

	insertedCodes := make([]string, 0, len(models))
	for _, m := range models {
		res := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "badge_code"}},
			DoNothing: true,
		}).Create(m)
		if res.Error != nil {
			return nil, fmt.Errorf("batch save user badges: %w", res.Error)
		}
		if res.RowsAffected > 0 {
			insertedCodes = append(insertedCodes, m.BadgeCode)
		}
	}

	return insertedCodes, nil
}
