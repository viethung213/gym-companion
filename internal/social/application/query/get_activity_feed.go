package query

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/service"
)

type GetActivityFeedQuery struct {
	CurrentUserID string
	PageSize      int
	Cursor        string
}

type GetActivityFeedResult struct {
	Items      []*FeedItemDTO
	NextCursor string
}

type GetActivityFeedOption func(*GetActivityFeedHandler)

// WithDiversityService configures a custom FeedDiversityService.
func WithDiversityService(svc *service.FeedDiversityService) GetActivityFeedOption {
	return func(h *GetActivityFeedHandler) {
		if svc != nil {
			h.diversityService = svc
		}
	}
}

// WithFeedBloomFilter configures a FeedBloomFilter for seen deduplication.
func WithFeedBloomFilter(bf port.FeedBloomFilter) GetActivityFeedOption {
	return func(h *GetActivityFeedHandler) {
		h.bloomFilter = bf
	}
}

// WithTrendingPoolCache configures a TrendingPoolCache for trending items blending.
func WithTrendingPoolCache(tc port.TrendingPoolCache) GetActivityFeedOption {
	return func(h *GetActivityFeedHandler) {
		h.trendingCache = tc
	}
}

type GetActivityFeedHandler struct {
	followRepo       repository.FollowRepository
	feedItemRepo     repository.FeedItemRepository
	interactionRepo  repository.InteractionRepository
	userSnapshotRepo repository.UserSnapshotRepository
	diversityService *service.FeedDiversityService
	bloomFilter      port.FeedBloomFilter
	trendingCache    port.TrendingPoolCache
}

func NewGetActivityFeedHandler(
	followRepo repository.FollowRepository,
	feedItemRepo repository.FeedItemRepository,
	interactionRepo repository.InteractionRepository,
	userSnapshotRepo repository.UserSnapshotRepository,
	opts ...GetActivityFeedOption,
) *GetActivityFeedHandler {
	h := &GetActivityFeedHandler{
		followRepo:       followRepo,
		feedItemRepo:     feedItemRepo,
		interactionRepo:  interactionRepo,
		userSnapshotRepo: userSnapshotRepo,
		diversityService: service.NewFeedDiversityService(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (h *GetActivityFeedHandler) Handle(ctx context.Context, q GetActivityFeedQuery) (*GetActivityFeedResult, error) {
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 50 {
		q.PageSize = 50
	}

	followingIDs, err := h.followRepo.GetFollowingIDs(ctx, q.CurrentUserID)
	if err != nil {
		return nil, fmt.Errorf("get following ids: %w", err)
	}

	// Tác giả bao gồm chính user + những người user follow
	authorIDs := append([]string{q.CurrentUserID}, followingIDs...)

	// Kích thước lấy ứng viên (over-fetching buffer cho Tầng 1)
	fetchLimit := q.PageSize
	if q.Cursor == "" {
		fetchLimit = min(q.PageSize*2, 60)
	}

	feedItems, followingRepoCursor, fetchErr := h.feedItemRepo.GetByAuthorIDs(ctx, authorIDs, fetchLimit, q.Cursor)
	if fetchErr != nil {
		return nil, fmt.Errorf("fetch feed items: %w", fetchErr)
	}

	// Tầng 1: Lọc bài viết đã xem qua Bloom Filter khi refresh (cursor rỗng)
	if q.Cursor == "" && h.bloomFilter != nil && len(feedItems) > 0 {
		feedItems = h.filterUnseenItems(ctx, q.CurrentUserID, feedItems)
	}

	totalAvailableFollowing := len(feedItems)

	// Tỷ lệ 80/20: Mục tiêu lấy 80% từ Following, phần còn thiếu lấy từ Trending/Discovery
	followingTarget := q.PageSize
	if h.trendingCache != nil && q.PageSize >= 5 {
		followingTarget = int(float64(q.PageSize) * 0.8)
	}

	takeFollowing := min(len(feedItems), followingTarget)
	followingItems := feedItems[:takeFollowing]

	// Tầng 2: Phần còn thiếu để đủ PageSize -> lấy từ Redis Trending Pool
	needed := q.PageSize - len(followingItems)
	existingMap := make(map[string]bool, len(feedItems))
	for _, item := range feedItems {
		existingMap[item.ID()] = true
	}

	var trendingItems []*aggregate.FeedItem
	if needed > 0 && h.trendingCache != nil {
		trendingItems = h.fetchTrendingItems(ctx, q.CurrentUserID, existingMap, needed)
	}

	// Quỹ dự phòng: Nếu Trending không đủ `needed`, lấy nốt các bài còn lại của Following bù vào
	currentCount := len(followingItems) + len(trendingItems)
	if currentCount < q.PageSize && takeFollowing < len(feedItems) {
		remainingFollowing := feedItems[takeFollowing:]
		deficit := q.PageSize - currentCount
		take := min(deficit, len(remainingFollowing))
		followingItems = append(followingItems, remainingFollowing[:take]...)
	}

	// Xen kẽ (Interleave) bài Following và Trending theo nhịp đều đặn
	combined := interleaveFeed(followingItems, trendingItems)

	// Tầng 3: Fallback sang bài viết công khai gần nhất (Public Stream) nếu vẫn chưa đủ PageSize
	var pubCursor string
	if len(combined) < q.PageSize {
		neededPublic := q.PageSize - len(combined)
		publicItems, pCursor, pubErr := h.feedItemRepo.GetPublicFeedItems(ctx, neededPublic, q.Cursor)
		if pubErr == nil && len(publicItems) > 0 {
			if q.Cursor == "" && h.bloomFilter != nil {
				publicItems = h.filterUnseenItems(ctx, q.CurrentUserID, publicItems)
			}
			combined = appendUniqueItems(combined, publicItems)
			pubCursor = pCursor
			if pubCursor == "" && len(publicItems) > 0 {
				pubCursor = publicItems[len(publicItems)-1].CreatedAt().Format("2006-01-02T15:04:05.000000Z07:00")
			}
		}
	}

	// Tầng 4: Xả van an toàn (Safety Valve / Never-Empty Fallback)
	// Nếu người dùng đã xem hết 100% nội dung (cả bạn bè, trending và public),
	// bỏ qua bộ lọc Bloom Filter để lấy các bài viết công khai mới nhất, tránh màn hình trắng trơn.
	if len(combined) == 0 {
		emergencyItems, emCursor, emErr := h.feedItemRepo.GetPublicFeedItems(ctx, q.PageSize, q.Cursor)
		if emErr == nil && len(emergencyItems) == 0 && q.Cursor != "" {
			// Nếu phân trang cũ đã cạn kiệt, quay về đầu dòng thời gian công khai
			emergencyItems, emCursor, emErr = h.feedItemRepo.GetPublicFeedItems(ctx, q.PageSize, "")
		}
		if emErr == nil && len(emergencyItems) > 0 {
			combined = emergencyItems
			pubCursor = emCursor
			if pubCursor == "" {
				pubCursor = emergencyItems[len(emergencyItems)-1].CreatedAt().Format("2006-01-02T15:04:05.000000Z07:00")
			}
		}
	}

	// Tầng cuối: Áp dụng thuật toán Author Diversity (không quá 2 bài liên tiếp của cùng 1 tác giả)
	feedItems = combined
	if len(feedItems) > 0 {
		feedItems = h.diversityService.ReorderAndDiversify(feedItems, service.FeedDiversityConfig{
			MaxConsecutiveAuthor: 2,
			TargetSize:           q.PageSize,
		})
	}

	// Giới hạn kết quả theo đúng PageSize
	if len(feedItems) > q.PageSize {
		feedItems = feedItems[:q.PageSize]
	}

	// Tính nextCursor:
	// QUAN TRỌNG: Cursor phân trang phải bám theo dòng thời gian của bài Bạn bè (Following Stream),
	// tuyệt đối không lấy thời gian của bài Trending (vì bài Trending là bài hot 48h ngẫu nhiên,
	// nếu lấy thời gian bài Trending sẽ làm trượt cursor và bỏ sót bài của bạn bè).
	var nextCursor string
	consumedFollowing := len(followingItems)
	hasMoreFollowing := consumedFollowing < totalAvailableFollowing || followingRepoCursor != ""
	if consumedFollowing > 0 && hasMoreFollowing {
		lastFollowing := followingItems[consumedFollowing-1]
		nextCursor = lastFollowing.CreatedAt().Format("2006-01-02T15:04:05.000000Z07:00")
	} else if pubCursor != "" {
		nextCursor = pubCursor
	}

	// Ghi nhận các bài viết đã trả về vào Bloom Filter
	if h.bloomFilter != nil && len(feedItems) > 0 && q.CurrentUserID != "" {
		returnedIDs := make([]string, len(feedItems))
		for i, item := range feedItems {
			returnedIDs[i] = item.ID()
		}
		_ = h.bloomFilter.AddSeen(ctx, q.CurrentUserID, returnedIDs, 3*24*time.Hour)
	}

	// Lấy thông tin user (Tên, Avatar) và tương tác của current user theo batch
	var distinctUserIDs []string
	seen := make(map[string]bool)
	feedItemIDs := make([]string, 0, len(feedItems))
	for _, item := range feedItems {
		feedItemIDs = append(feedItemIDs, item.ID())
		if !seen[item.UserID()] {
			seen[item.UserID()] = true
			distinctUserIDs = append(distinctUserIDs, item.UserID())
		}
	}

	userInfoMap, _ := h.userSnapshotRepo.GetByIDs(ctx, distinctUserIDs)

	var userReactionMap map[string]*entity.Reaction
	if q.CurrentUserID != "" && len(feedItemIDs) > 0 {
		userReactionMap, _ = h.interactionRepo.GetUserReactions(ctx, q.CurrentUserID, feedItemIDs)
	}

	result := make([]*FeedItemDTO, 0, len(feedItems))
	for _, item := range feedItems {
		authorName := ""
		avatarURL := entity.DefaultAvatarURL
		if u, ok := userInfoMap[item.UserID()]; ok && u != nil {
			authorName = u.FullName()
			if u.AvatarURL() != "" {
				avatarURL = u.AvatarURL()
			}
		}

		userReaction := ""
		if userReactionMap != nil {
			if r, ok := userReactionMap[item.ID()]; ok && r != nil {
				userReaction = r.ReactionType().String()
			}
		}

		result = append(result, &FeedItemDTO{
			ID:              item.ID(),
			UserID:          item.UserID(),
			AuthorName:      authorName,
			AuthorAvatarURL: avatarURL,
			ItemType:        item.ItemType().String(),
			Caption:         item.Caption(),
			MediaURLs:       item.MediaURLs(),
			Visibility:      item.Visibility().String(),
			ReactionCount:   item.ReactionCount(),
			CommentCount:    item.CommentCount(),
			UserReaction:    userReaction,
			CreatedAt:       item.CreatedAt(),
			UpdatedAt:       item.UpdatedAt(),
			WorkoutData:     item.WorkoutData(),
		})
	}

	return &GetActivityFeedResult{
		Items:      result,
		NextCursor: nextCursor,
	}, nil
}

func (h *GetActivityFeedHandler) filterUnseenItems(
	ctx context.Context,
	userID string,
	items []*aggregate.FeedItem,
) []*aggregate.FeedItem {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID()
	}

	unseenIDs, err := h.bloomFilter.FilterUnseen(ctx, userID, ids)
	if err != nil || len(unseenIDs) == len(items) {
		return items
	}

	unseenMap := make(map[string]bool, len(unseenIDs))
	for _, id := range unseenIDs {
		unseenMap[id] = true
	}

	filtered := make([]*aggregate.FeedItem, 0, len(unseenIDs))
	for _, item := range items {
		if unseenMap[item.ID()] {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func (h *GetActivityFeedHandler) fetchTrendingItems(
	ctx context.Context,
	userID string,
	excludeIDs map[string]bool,
	count int,
) []*aggregate.FeedItem {
	if h.trendingCache == nil || count <= 0 {
		return nil
	}

	poolIDs, err := h.trendingCache.GetTrendingPool(ctx)
	if err != nil || len(poolIDs) == 0 {
		return nil
	}

	candidateIDs := make([]string, 0, len(poolIDs))
	for _, id := range poolIDs {
		if !excludeIDs[id] {
			candidateIDs = append(candidateIDs, id)
		}
	}

	if len(candidateIDs) == 0 {
		return nil
	}

	var unseenIDs []string
	if h.bloomFilter != nil {
		var filterErr error
		unseenIDs, filterErr = h.bloomFilter.FilterUnseen(ctx, userID, candidateIDs)
		if filterErr != nil {
			unseenIDs = candidateIDs
		}
	} else {
		unseenIDs = candidateIDs
	}

	if len(unseenIDs) > count {
		unseenIDs = unseenIDs[:count]
	}

	if len(unseenIDs) == 0 {
		return nil
	}

	hydrated, err := h.feedItemRepo.GetByIDs(ctx, unseenIDs)
	if err != nil || len(hydrated) == 0 {
		return nil
	}

	return hydrated
}

func interleaveFeed(following, trending []*aggregate.FeedItem) []*aggregate.FeedItem {
	if len(trending) == 0 {
		return following
	}
	if len(following) == 0 {
		return trending
	}

	total := len(following) + len(trending)
	result := make([]*aggregate.FeedItem, 0, total)

	fIdx, tIdx := 0, 0
	interval := len(following) / len(trending)
	if interval < 1 {
		interval = 1
	}

	countSinceLastTrending := 0
	for fIdx < len(following) || tIdx < len(trending) {
		if tIdx < len(trending) && (countSinceLastTrending >= interval || fIdx >= len(following)) {
			result = append(result, trending[tIdx])
			tIdx++
			countSinceLastTrending = 0
		} else if fIdx < len(following) {
			result = append(result, following[fIdx])
			fIdx++
			countSinceLastTrending++
		}
	}

	return result
}

func appendUniqueItems(base, additions []*aggregate.FeedItem) []*aggregate.FeedItem {
	existing := make(map[string]bool, len(base))
	for _, item := range base {
		existing[item.ID()] = true
	}

	result := make([]*aggregate.FeedItem, len(base), len(base)+len(additions))
	copy(result, base)

	for _, item := range additions {
		if !existing[item.ID()] {
			existing[item.ID()] = true
			result = append(result, item)
		}
	}

	return result
}
