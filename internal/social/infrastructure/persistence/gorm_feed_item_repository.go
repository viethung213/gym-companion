package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"gorm.io/gorm"
)

type GormFeedItemRepository struct {
	db *gorm.DB
}

var _ repository.FeedItemRepository = (*GormFeedItemRepository)(nil)

func NewGormFeedItemRepository(db *gorm.DB) *GormFeedItemRepository {
	return &GormFeedItemRepository{db: db}
}

func (r *GormFeedItemRepository) Create(ctx context.Context, item *aggregate.FeedItem) error {
	m := ToPersistenceFeedItem(item)
	if err := getDB(ctx, r.db).Create(m).Error; err != nil {
		return fmt.Errorf("create feed item: %w", err)
	}
	return nil
}

func (r *GormFeedItemRepository) GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error) {
	var m FeedItemModel
	err := getDB(ctx, r.db).Where("id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find feed item by id: %w", err)
	}
	return ToDomainFeedItem(&m)
}

func (r *GormFeedItemRepository) Delete(ctx context.Context, id, userID string) error {
	err := getDB(ctx, r.db).Where("id = ? AND user_id = ?", id, userID).Delete(&FeedItemModel{}).Error
	if err != nil {
		return fmt.Errorf("delete feed item: %w", err)
	}
	return nil
}

func (r *GormFeedItemRepository) GetByUserID(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	var models []FeedItemModel
	query := getDB(ctx, r.db).Where("user_id = ?", userID)

	if cursor != "" {
		if t, err := time.Parse("2006-01-02T15:04:05.000000Z07:00", cursor); err == nil {
			query = query.Where("created_at < ?", t)
		}
	}

	if err := query.Order("created_at DESC").Limit(limit).Find(&models).Error; err != nil {
		return nil, "", fmt.Errorf("find user feed items: %w", err)
	}

	var res []*aggregate.FeedItem
	for i := range models {
		domainItem, err := ToDomainFeedItem(&models[i])
		if err == nil {
			res = append(res, domainItem)
		}
	}

	nextCursor := ""
	if len(models) == limit {
		nextCursor = models[len(models)-1].CreatedAt.Format("2006-01-02T15:04:05.000000Z07:00")
	}

	return res, nextCursor, nil
}

func (r *GormFeedItemRepository) GetByAuthorIDs(ctx context.Context, authorIDs []string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	if len(authorIDs) == 0 {
		return nil, "", nil
	}

	var models []FeedItemModel
	query := getDB(ctx, r.db).Where("user_id IN ?", authorIDs)

	if cursor != "" {
		if t, err := time.Parse("2006-01-02T15:04:05.000000Z07:00", cursor); err == nil {
			query = query.Where("created_at < ?", t)
		}
	}

	if err := query.Order("created_at DESC").Limit(limit).Find(&models).Error; err != nil {
		return nil, "", fmt.Errorf("find feed items by author ids: %w", err)
	}

	var res []*aggregate.FeedItem
	for i := range models {
		domainItem, err := ToDomainFeedItem(&models[i])
		if err == nil {
			res = append(res, domainItem)
		}
	}

	nextCursor := ""
	if len(models) == limit {
		nextCursor = models[len(models)-1].CreatedAt.Format("2006-01-02T15:04:05.000000Z07:00")
	}

	return res, nextCursor, nil
}

func (r *GormFeedItemRepository) CountByUserID(ctx context.Context, userID string) (int32, error) {
	var count int64
	err := getDB(ctx, r.db).Model(&FeedItemModel{}).Where("user_id = ?", userID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count user feed items: %w", err)
	}
	return int32(count), nil
}

func (r *GormFeedItemRepository) UpdateReactionCount(ctx context.Context, id string, delta int) error {
	expr := "reaction_count + ?"
	if delta < 0 {
		expr = "GREATEST(reaction_count + ?, 0)"
	}
	return getDB(ctx, r.db).Model(&FeedItemModel{}).
		Where("id = ?", id).
		UpdateColumn("reaction_count", gorm.Expr(expr, delta)).Error
}

func (r *GormFeedItemRepository) UpdateCommentCount(ctx context.Context, id string, delta int) error {
	expr := "comment_count + ?"
	if delta < 0 {
		expr = "GREATEST(comment_count + ?, 0)"
	}
	return getDB(ctx, r.db).Model(&FeedItemModel{}).
		Where("id = ?", id).
		UpdateColumn("comment_count", gorm.Expr(expr, delta)).Error
}
