package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormLeaderboardRepository implements repository.LeaderboardRepository.
type GormLeaderboardRepository struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ repository.LeaderboardRepository = (*GormLeaderboardRepository)(nil)

// NewGormLeaderboardRepository constructs a new GormLeaderboardRepository.
func NewGormLeaderboardRepository(db *gorm.DB) *GormLeaderboardRepository {
	return &GormLeaderboardRepository{db: db}
}

type leaderboardRow struct {
	UserID      string    `gorm:"column:user_id"`
	Xp          int64     `gorm:"column:xp"`
	Level       int32     `gorm:"column:level"`
	DisplayName *string   `gorm:"column:display_name"`
	AvatarURL   *string   `gorm:"column:avatar_url"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

// GetGlobalTop queries the Top users ordered by (xp DESC, updated_at ASC, user_id ASC)
// joined with local snapshots to avoid cross-schema joins.
func (r *GormLeaderboardRepository) GetGlobalTop(
	ctx context.Context,
	limit, offset int,
) ([]*entity.LeaderboardUser, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	db := GetDB(ctx, r.db)

	var rows []leaderboardRow
	err := db.Table("gamification.user_xp u").
		Select("u.user_id, u.xp, u.level, u.updated_at, s.display_name, s.avatar_url").
		Joins("LEFT JOIN gamification.user_snapshots s ON u.user_id = s.user_id").
		Order("u.xp DESC, u.updated_at ASC, u.user_id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("query global top leaderboard: %w", err)
	}

	result := make([]*entity.LeaderboardUser, 0, len(rows))
	for i, row := range rows {
		displayName := "Gymer"
		if row.DisplayName != nil && *row.DisplayName != "" {
			displayName = *row.DisplayName
		}
		avatarURL := ""
		if row.AvatarURL != nil {
			avatarURL = *row.AvatarURL
		}

		result = append(result, &entity.LeaderboardUser{
			Rank:        int64(offset + i + 1),
			UserID:      row.UserID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
			Xp:          row.Xp,
			Level:       row.Level,
			UpdatedAt:   row.UpdatedAt,
		})
	}

	return result, int64(len(result)), nil
}

// GetUserRank calculates a specific user's rank using the indexed tie-breaking ordering.
func (r *GormLeaderboardRepository) GetUserRank(
	ctx context.Context,
	userID string,
) (int64, *entity.LeaderboardUser, error) {
	db := GetDB(ctx, r.db)

	var userRow leaderboardRow
	err := db.Table("gamification.user_xp u").
		Select("u.user_id, u.xp, u.level, u.updated_at, s.display_name, s.avatar_url").
		Joins("LEFT JOIN gamification.user_snapshots s ON u.user_id = s.user_id").
		Where("u.user_id = ?", userID).
		Take(&userRow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("find user for rank: %w", err)
	}

	var higherCount int64
	rankQuery := `
		SELECT COUNT(*)
		FROM gamification.user_xp
		WHERE (xp > ?)
		   OR (xp = ? AND updated_at < ?)
		   OR (xp = ? AND updated_at = ? AND user_id < ?)
	`
	err = db.Raw(
		rankQuery,
		userRow.Xp,
		userRow.Xp, userRow.UpdatedAt,
		userRow.Xp, userRow.UpdatedAt, userRow.UserID,
	).Scan(&higherCount).Error
	if err != nil {
		return 0, nil, fmt.Errorf("calculate user rank: %w", err)
	}

	rank := higherCount + 1

	displayName := "Gymer"
	if userRow.DisplayName != nil && *userRow.DisplayName != "" {
		displayName = *userRow.DisplayName
	}
	avatarURL := ""
	if userRow.AvatarURL != nil {
		avatarURL = *userRow.AvatarURL
	}

	leaderboardUser := &entity.LeaderboardUser{
		Rank:        rank,
		UserID:      userRow.UserID,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		Xp:          userRow.Xp,
		Level:       userRow.Level,
		UpdatedAt:   userRow.UpdatedAt,
	}

	return rank, leaderboardUser, nil
}

// SaveSnapshot upserts user display information into gamification.user_snapshots.
func (r *GormLeaderboardRepository) SaveSnapshot(ctx context.Context, snapshot *entity.UserSnapshot) error {
	if snapshot == nil {
		return errors.New("nil user snapshot")
	}

	model := &UserSnapshotModel{
		UserID:      snapshot.UserID,
		DisplayName: snapshot.DisplayName,
		AvatarURL:   snapshot.AvatarURL,
		UpdatedAt:   snapshot.UpdatedAt,
	}

	db := GetDB(ctx, r.db)
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		UpdateAll: true,
	}).Create(model).Error
	if err != nil {
		return fmt.Errorf("save user snapshot: %w", err)
	}
	return nil
}

// GetSnapshot retrieves the local snapshot for a user.
func (r *GormLeaderboardRepository) GetSnapshot(ctx context.Context, userID string) (*entity.UserSnapshot, error) {
	db := GetDB(ctx, r.db)
	var model UserSnapshotModel
	err := db.Where("user_id = ?", userID).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find user snapshot: %w", err)
	}

	return &entity.UserSnapshot{
		UserID:      model.UserID,
		DisplayName: model.DisplayName,
		AvatarURL:   model.AvatarURL,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}
