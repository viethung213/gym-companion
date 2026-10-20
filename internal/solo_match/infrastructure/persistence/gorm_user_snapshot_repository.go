package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMUserSnapshotRepository struct {
	db *gorm.DB
}

var _ repository.UserSnapshotRepository = (*GORMUserSnapshotRepository)(nil)

func NewGORMUserSnapshotRepository(db *gorm.DB) *GORMUserSnapshotRepository {
	return &GORMUserSnapshotRepository{db: db}
}

func (r *GORMUserSnapshotRepository) Upsert(ctx context.Context, snapshot *entity.UserSnapshot) error {
	db := GetDB(ctx, r.db)
	model := FromDomainUserSnapshot(snapshot)

	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"display_name", "avatar_url", "synced_at",
		}),
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("upsert user snapshot %s: %w", snapshot.UserID(), err)
	}

	return nil
}

func (r *GORMUserSnapshotRepository) FindByUserID(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	db := GetDB(ctx, r.db)
	var model UserSnapshotModel

	err := db.First(&model, "user_id = ?", userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user snapshot by id %s: %w", userID, err)
	}

	return model.ToDomain()
}

func (r *GORMUserSnapshotRepository) FindByUserIDs(ctx context.Context, userIDs []string) (map[string]*entity.UserSnapshot, error) {
	if len(userIDs) == 0 {
		return make(map[string]*entity.UserSnapshot), nil
	}

	db := GetDB(ctx, r.db)
	var models []UserSnapshotModel

	err := db.Where("user_id IN ?", userIDs).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("find user snapshots by ids: %w", err)
	}

	res := make(map[string]*entity.UserSnapshot, len(models))
	for i := range models {
		domainObj, err := models[i].ToDomain()
		if err != nil {
			return nil, fmt.Errorf("convert user snapshot %s: %w", models[i].UserID, err)
		}
		res[models[i].UserID] = domainObj
	}

	return res, nil
}
