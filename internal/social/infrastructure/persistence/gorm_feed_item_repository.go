package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	return strings.ReplaceAll(s, "_", "\\_")
}

//nolint:gocritic // filter parameter struct passed by value matching repository port
func (r *GormFeedItemRepository) SearchPosts(
	ctx context.Context,
	filter repository.SearchPostsFilter,
) ([]*aggregate.FeedItem, string, int32, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	db := getDB(ctx, r.db).Model(&FeedItemModel{})

	// 1. Lọc theo từ khóa trong caption (sử dụng ILIKE để ăn GIN index trên Postgres)
	if query := strings.TrimSpace(filter.Query); query != "" {
		likeOp := "ILIKE"
		if r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() == "sqlite" {
			likeOp = "LIKE"
		}
		searchTerm := "%" + escapeLikePattern(query) + "%"
		db = db.Where(fmt.Sprintf("caption %s ? ESCAPE '\\'", likeOp), searchTerm)
	}

	// 2. Lọc theo tác giả cụ thể nếu có
	if authorID := strings.TrimSpace(filter.AuthorID); authorID != "" {
		db = db.Where("user_id = ?", authorID)
	}

	// 3. Lọc theo loại bài viết (chỉ chấp nhận domain string đã chuẩn hóa)
	if itemType := strings.TrimSpace(filter.ItemType); itemType != "" {
		db = db.Where("item_type = ?", itemType)
	}

	// 4. Lọc quyền riêng tư (Visibility Access Control) - Dùng EXISTS thay vì IN để tối ưu hiệu năng
	if filter.ViewerID == "" {
		// Chưa đăng nhập chỉ thấy bài PUBLIC
		db = db.Where("visibility = ?", "PUBLIC")
	} else {
		// Đã đăng nhập: thấy bài PUBLIC, bài của chính mình, hoặc bài FOLLOWERS_ONLY của người mình đang follow
		subQuery := "visibility = 'PUBLIC' OR user_id = ? OR " +
			"(visibility = 'FOLLOWERS_ONLY' AND EXISTS " +
			"(SELECT 1 FROM social.follows f WHERE f.follower_id = ? AND f.following_id = social.feed_items.user_id))"
		db = db.Where(subQuery, filter.ViewerID, filter.ViewerID)
	}

	// 5. Đếm tổng số kết quả phù hợp (tách Session để tránh bleed state sang Find)
	countDB := db.Session(&gorm.Session{})
	var total int64
	if err := countDB.Count(&total).Error; err != nil {
		return nil, "", 0, fmt.Errorf("count search posts: %w", err)
	}

	// 6. Phân trang theo Compound Cursor (created_at DESC, id DESC)
	queryDB := db.Session(&gorm.Session{})
	if filter.Cursor != "" {
		if strings.Contains(filter.Cursor, "#") {
			parts := strings.SplitN(filter.Cursor, "#", 2)
			if t, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				queryDB = queryDB.Where("created_at < ? OR (created_at = ? AND id < ?)", t, t, parts[1])
			}
		} else {
			if t, err := time.Parse("2006-01-02T15:04:05.000000Z07:00", filter.Cursor); err == nil {
				queryDB = queryDB.Where("created_at < ?", t)
			} else if t, err := time.Parse(time.RFC3339Nano, filter.Cursor); err == nil {
				queryDB = queryDB.Where("created_at < ?", t)
			} else if t, err := time.Parse(time.RFC3339, filter.Cursor); err == nil {
				queryDB = queryDB.Where("created_at < ?", t)
			}
		}
	}

	var models []FeedItemModel
	if err := queryDB.Order("created_at DESC, id DESC").Limit(limit).Find(&models).Error; err != nil {
		return nil, "", 0, fmt.Errorf("find search posts: %w", err)
	}

	res := make([]*aggregate.FeedItem, 0, len(models))
	for i := range models {
		domainItem, err := ToDomainFeedItem(&models[i])
		if err != nil {
			return nil, "", 0, fmt.Errorf("corrupt feed item data %s: %w", models[i].ID, err)
		}
		res = append(res, domainItem)
	}

	nextCursor := ""
	if len(models) == limit {
		last := &models[len(models)-1]
		nextCursor = fmt.Sprintf("%s#%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}

	return res, nextCursor, int32(total), nil
}
