package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormUserSnapshotRepository implements repository.UserSnapshotRepository on
// gamification.user_snapshots.
type GormUserSnapshotRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ repository.UserSnapshotRepository = (*GormUserSnapshotRepository)(nil)

// NewGormUserSnapshotRepository constructs a new GormUserSnapshotRepository.
func NewGormUserSnapshotRepository(db *gorm.DB) *GormUserSnapshotRepository {
	return &GormUserSnapshotRepository{db: db}
}

// SaveSnapshot upserts user display information into gamification.user_snapshots.
func (r *GormUserSnapshotRepository) SaveSnapshot(
	ctx context.Context,
	snapshot *entity.UserSnapshot,
) error {
	if snapshot == nil {
		return errors.New("nil user snapshot")
	}

	model := &UserSnapshotModel{
		UserID:      snapshot.UserID,
		DisplayName: snapshot.DisplayName,
		AvatarURL:   snapshot.AvatarURL,
		UpdatedAt:   snapshot.UpdatedAt,
	}

	err := GetDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		UpdateAll: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save user snapshot: %w", err)
	}
	return nil
}

// GetSnapshot retrieves the local snapshot for a user, or nil when none exists.
func (r *GormUserSnapshotRepository) GetSnapshot(
	ctx context.Context,
	userID string,
) (*entity.UserSnapshot, error) {
	var model UserSnapshotModel
	err := GetDB(ctx, r.db).Where("user_id = ?", userID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user snapshot: %w", err)
	}
	return snapshotModelToDomain(&model), nil
}

// GetSnapshots retrieves snapshots for userIDs in a single query, keyed by user ID.
func (r *GormUserSnapshotRepository) GetSnapshots(
	ctx context.Context,
	userIDs []string,
) (map[string]*entity.UserSnapshot, error) {
	result := make(map[string]*entity.UserSnapshot, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	var models []UserSnapshotModel
	if err := GetDB(ctx, r.db).Where("user_id IN ?", userIDs).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("find user snapshots: %w", err)
	}
	for i := range models {
		result[models[i].UserID] = snapshotModelToDomain(&models[i])
	}
	return result, nil
}

func snapshotModelToDomain(m *UserSnapshotModel) *entity.UserSnapshot {
	return &entity.UserSnapshot{
		UserID:      m.UserID,
		DisplayName: m.DisplayName,
		AvatarURL:   m.AvatarURL,
		UpdatedAt:   m.UpdatedAt,
	}
}
