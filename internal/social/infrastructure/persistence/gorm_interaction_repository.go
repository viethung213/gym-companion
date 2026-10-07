package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormInteractionRepository struct {
	db *gorm.DB
}

var _ repository.InteractionRepository = (*GormInteractionRepository)(nil)

func NewGormInteractionRepository(db *gorm.DB) *GormInteractionRepository {
	return &GormInteractionRepository{db: db}
}

func (r *GormInteractionRepository) SaveReaction(ctx context.Context, reaction *entity.Reaction) error {
	m := ToPersistenceReaction(reaction)
	err := getDB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "feed_item_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"reaction_type"}),
	}).Create(m).Error
	if err != nil {
		return fmt.Errorf("save reaction: %w", err)
	}
	return nil
}

func (r *GormInteractionRepository) DeleteReaction(ctx context.Context, userID, feedItemID string) error {
	err := getDB(ctx, r.db).Where("user_id = ? AND feed_item_id = ?", userID, feedItemID).
		Delete(&ReactionModel{}).Error
	if err != nil {
		return fmt.Errorf("delete reaction: %w", err)
	}
	return nil
}

func (r *GormInteractionRepository) GetReaction(ctx context.Context, userID, feedItemID string) (*entity.Reaction, error) {
	var m ReactionModel
	err := getDB(ctx, r.db).Where("user_id = ? AND feed_item_id = ?", userID, feedItemID).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find reaction: %w", err)
	}
	return ToDomainReaction(&m)
}

func (r *GormInteractionRepository) GetReactionsByFeedItem(ctx context.Context, feedItemID string) ([]*entity.Reaction, error) {
	var models []ReactionModel
	err := getDB(ctx, r.db).Where("feed_item_id = ?", feedItemID).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("find reactions by feed item: %w", err)
	}
	var res []*entity.Reaction
	for i := range models {
		item, err := ToDomainReaction(&models[i])
		if err == nil {
			res = append(res, item)
		}
	}
	return res, nil
}

func (r *GormInteractionRepository) AddComment(ctx context.Context, comment *entity.Comment) error {
	m := ToPersistenceComment(comment)
	if err := getDB(ctx, r.db).Create(m).Error; err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
}

func (r *GormInteractionRepository) GetCommentByID(ctx context.Context, id string) (*entity.Comment, error) {
	var m CommentModel
	err := getDB(ctx, r.db).Where("id = ?", id).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find comment by id: %w", err)
	}
	return ToDomainComment(&m)
}

func (r *GormInteractionRepository) DeleteComment(ctx context.Context, id, userID string) error {
	err := getDB(ctx, r.db).Where("id = ? AND user_id = ?", id, userID).Delete(&CommentModel{}).Error
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}

func (r *GormInteractionRepository) ListComments(
	ctx context.Context,
	feedItemID string,
	limit int,
	cursor string,
) (items []*entity.Comment, nextCursor string, total int32, err error) {
	var models []CommentModel
	query := getDB(ctx, r.db).Where("feed_item_id = ?", feedItemID)

	var totalCount int64
	if countErr := query.Model(&CommentModel{}).Count(&totalCount).Error; countErr != nil {
		return nil, "", 0, fmt.Errorf("count comments: %w", countErr)
	}

	if cursor != "" {
		if t, parseErr := time.Parse("2006-01-02T15:04:05.000000Z07:00", cursor); parseErr == nil {
			query = query.Where("created_at > ?", t)
		}
	}

	if findErr := query.Order("created_at ASC").Limit(limit).Find(&models).Error; findErr != nil {
		return nil, "", 0, fmt.Errorf("find comments: %w", findErr)
	}

	res := make([]*entity.Comment, 0, len(models))
	for i := range models {
		item, domainErr := ToDomainComment(&models[i])
		if domainErr == nil {
			res = append(res, item)
		}
	}

	cursorStr := ""
	if len(models) == limit {
		cursorStr = models[len(models)-1].CreatedAt.Format("2006-01-02T15:04:05.000000Z07:00")
	}

	return res, cursorStr, int32(totalCount), nil
}
