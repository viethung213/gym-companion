package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormFollowRepository struct {
	db *gorm.DB
}

var _ repository.FollowRepository = (*GormFollowRepository)(nil)

func NewGormFollowRepository(db *gorm.DB) *GormFollowRepository {
	return &GormFollowRepository{db: db}
}

func (r *GormFollowRepository) Follow(ctx context.Context, follow *aggregate.Follow) error {
	m := ToPersistenceFollow(follow)
	err := getDB(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).Create(m).Error
	if err != nil {
		return fmt.Errorf("create follow: %w", err)
	}
	return nil
}

func (r *GormFollowRepository) Unfollow(ctx context.Context, followerID, followingID string) error {
	err := getDB(ctx, r.db).Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Delete(&FollowModel{}).Error
	if err != nil {
		return fmt.Errorf("delete follow: %w", err)
	}
	return nil
}

func (r *GormFollowRepository) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	var count int64
	err := getDB(ctx, r.db).Model(&FollowModel{}).
		Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check is following: %w", err)
	}
	return count > 0, nil
}

func (r *GormFollowRepository) GetFollowers(
	ctx context.Context,
	userID string,
	limit int,
	cursor string,
) (items []*aggregate.Follow, nextCursor string, total int32, err error) {
	var models []FollowModel
	query := getDB(ctx, r.db).Where("following_id = ?", userID)

	var totalCount int64
	if countErr := query.Model(&FollowModel{}).Count(&totalCount).Error; countErr != nil {
		return nil, "", 0, fmt.Errorf("count followers: %w", countErr)
	}

	if cursor != "" {
		if t, parseErr := time.Parse("2006-01-02T15:04:05.000000Z07:00", cursor); parseErr == nil {
			query = query.Where("created_at < ?", t)
		}
	}

	if findErr := query.Order("created_at DESC").Limit(limit).Find(&models).Error; findErr != nil {
		return nil, "", 0, fmt.Errorf("find followers: %w", findErr)
	}

	res := make([]*aggregate.Follow, 0, len(models))
	for i := range models {
		domainItem, domainErr := ToDomainFollow(&models[i])
		if domainErr == nil {
			res = append(res, domainItem)
		}
	}

	cursorStr := ""
	if len(models) == limit {
		cursorStr = models[len(models)-1].CreatedAt.Format("2006-01-02T15:04:05.000000Z07:00")
	}

	return res, cursorStr, int32(totalCount), nil
}

func (r *GormFollowRepository) GetFollowing(
	ctx context.Context,
	userID string,
	limit int,
	cursor string,
) (items []*aggregate.Follow, nextCursor string, total int32, err error) {
	var models []FollowModel
	query := getDB(ctx, r.db).Where("follower_id = ?", userID)

	var totalCount int64
	if countErr := query.Model(&FollowModel{}).Count(&totalCount).Error; countErr != nil {
		return nil, "", 0, fmt.Errorf("count following: %w", countErr)
	}

	if cursor != "" {
		if t, parseErr := time.Parse("2006-01-02T15:04:05.000000Z07:00", cursor); parseErr == nil {
			query = query.Where("created_at < ?", t)
		}
	}

	if findErr := query.Order("created_at DESC").Limit(limit).Find(&models).Error; findErr != nil {
		return nil, "", 0, fmt.Errorf("find following: %w", findErr)
	}

	res := make([]*aggregate.Follow, 0, len(models))
	for i := range models {
		domainItem, domainErr := ToDomainFollow(&models[i])
		if domainErr == nil {
			res = append(res, domainItem)
		}
	}

	cursorStr := ""
	if len(models) == limit {
		cursorStr = models[len(models)-1].CreatedAt.Format("2006-01-02T15:04:05.000000Z07:00")
	}

	return res, cursorStr, int32(totalCount), nil
}

func (r *GormFollowRepository) GetFollowingIDs(ctx context.Context, followerID string) ([]string, error) {
	var ids []string
	err := getDB(ctx, r.db).Model(&FollowModel{}).
		Where("follower_id = ?", followerID).
		Pluck("following_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("pluck following ids: %w", err)
	}
	return ids, nil
}

func (r *GormFollowRepository) CountFollowers(ctx context.Context, userID string) (int32, error) {
	var count int64
	err := getDB(ctx, r.db).Model(&FollowModel{}).
		Where("following_id = ?", userID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count followers: %w", err)
	}
	return int32(count), nil
}

func (r *GormFollowRepository) CountFollowing(ctx context.Context, userID string) (int32, error) {
	var count int64
	err := getDB(ctx, r.db).Model(&FollowModel{}).
		Where("follower_id = ?", userID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count following: %w", err)
	}
	return int32(count), nil
}
