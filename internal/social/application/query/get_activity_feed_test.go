package query

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

type mockBloomFilterForQuery struct {
	seenMap   map[string]bool
	addedSeen []string
}

func (m *mockBloomFilterForQuery) FilterUnseen(ctx context.Context, userID string, itemIDs []string) ([]string, error) {
	var unseen []string
	for _, id := range itemIDs {
		if !m.seenMap[id] {
			unseen = append(unseen, id)
		}
	}
	return unseen, nil
}

func (m *mockBloomFilterForQuery) AddSeen(ctx context.Context, userID string, itemIDs []string, ttl time.Duration) error {
	if m.seenMap == nil {
		m.seenMap = make(map[string]bool)
	}
	for _, id := range itemIDs {
		m.seenMap[id] = true
		m.addedSeen = append(m.addedSeen, id)
	}
	return nil
}

type mockTrendingPoolCacheForQuery struct {
	trendingPool []string
}

func (m *mockTrendingPoolCacheForQuery) SaveTrendingPool(ctx context.Context, itemIDs []string) error {
	m.trendingPool = itemIDs
	return nil
}

func (m *mockTrendingPoolCacheForQuery) GetTrendingPool(ctx context.Context) ([]string, error) {
	return m.trendingPool, nil
}

func TestGetActivityFeedHandler(t *testing.T) {
	now := time.Now().UTC()
	p, _ := aggregate.NewPostItem("p-1", "user-friend", "Great workout", nil, vo.VisibilityPublic, 2, 1, now, now)
	metrics := vo.NewWorkoutMetrics("sess-1", "Leg day", 3600, 4000.0, 5, 15)
	a, _ := aggregate.NewWorkoutActivityItem("a-1", "user-friend", "Leg day", nil, metrics, vo.VisibilityPublic, 5, 2, now.Add(time.Minute), now.Add(time.Minute))

	followRepo := &queryMockFollowRepo{followingIDs: []string{"user-friend"}}
	feedItemRepo := &queryMockFeedItemRepo{items: []*aggregate.FeedItem{a, p}}
	r1, _ := entity.NewReaction("r-1", "user-me", "a-1", vo.ReactionTypeFire, now)
	interactionRepo := &queryMockInteractionRepo{
		reactions: []*entity.Reaction{r1},
	}
	userSnapshotRepo := &mockUserSnapshotRepo{
		users: map[string]*entity.UserSnapshot{
			"user-friend": entity.NewUserSnapshot("user-friend", "Friend Name", "http://avatar.jpg", "user", time.Time{}),
		},
	}

	handler := NewGetActivityFeedHandler(followRepo, feedItemRepo, interactionRepo, userSnapshotRepo)

	res, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Items) != 2 {
		t.Fatalf("expected 2 items in feed, got %d", len(res.Items))
	}
	if res.Items[0].ItemType != "WORKOUT_ACTIVITY" {
		t.Errorf("expected first item to be WORKOUT_ACTIVITY, got %s", res.Items[0].ItemType)
	}
	if res.Items[0].AuthorName != "Friend Name" {
		t.Errorf("expected author name 'Friend Name', got '%s'", res.Items[0].AuthorName)
	}
	if res.Items[0].UserReaction != "FIRE" {
		t.Errorf("expected user reaction 'FIRE', got '%s'", res.Items[0].UserReaction)
	}
	if res.Items[1].UserReaction != "" {
		t.Errorf("expected empty user reaction for item 2, got '%s'", res.Items[1].UserReaction)
	}
}

func TestGetActivityFeedHandler_Diversity(t *testing.T) {
	now := time.Now().UTC()
	// user-1 has 4 posts in a row, user-2 has 1 post
	p1, _ := aggregate.NewPostItem("p-1", "user-1", "Post 1", nil, vo.VisibilityPublic, 0, 0, now, now)
	p2, _ := aggregate.NewPostItem("p-2", "user-1", "Post 2", nil, vo.VisibilityPublic, 0, 0, now.Add(-time.Minute), now.Add(-time.Minute))
	p3, _ := aggregate.NewPostItem("p-3", "user-1", "Post 3", nil, vo.VisibilityPublic, 0, 0, now.Add(-2*time.Minute), now.Add(-2*time.Minute))
	p4, _ := aggregate.NewPostItem("p-4", "user-1", "Post 4", nil, vo.VisibilityPublic, 0, 0, now.Add(-3*time.Minute), now.Add(-3*time.Minute))
	p5, _ := aggregate.NewPostItem("p-5", "user-2", "Post 5", nil, vo.VisibilityPublic, 0, 0, now.Add(-4*time.Minute), now.Add(-4*time.Minute))

	followRepo := &queryMockFollowRepo{followingIDs: []string{"user-1", "user-2"}}
	feedItemRepo := &queryMockFeedItemRepo{items: []*aggregate.FeedItem{p1, p2, p3, p4, p5}}
	interactionRepo := &queryMockInteractionRepo{}
	userSnapshotRepo := &mockUserSnapshotRepo{}

	handler := NewGetActivityFeedHandler(followRepo, feedItemRepo, interactionRepo, userSnapshotRepo)

	res, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(res.Items))
	}

	// Order should be p-1, p-2 (max 2 for user-1), then p-5 (user-2), then p-3, p-4
	expectedIDs := []string{"p-1", "p-2", "p-5", "p-3", "p-4"}
	for i, item := range res.Items {
		if item.ID != expectedIDs[i] {
			t.Errorf("at index %d: expected %s, got %s", i, expectedIDs[i], item.ID)
		}
	}
}

func TestGetActivityFeedHandler_BloomFilterDeduplication(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := aggregate.NewPostItem("p-1", "user-1", "Post 1", nil, vo.VisibilityPublic, 0, 0, now, now)
	p2, _ := aggregate.NewPostItem("p-2", "user-1", "Post 2", nil, vo.VisibilityPublic, 0, 0, now.Add(-time.Minute), now.Add(-time.Minute))
	p3, _ := aggregate.NewPostItem("p-3", "user-1", "Post 3", nil, vo.VisibilityPublic, 0, 0, now.Add(-2*time.Minute), now.Add(-2*time.Minute))

	followRepo := &queryMockFollowRepo{followingIDs: []string{"user-1"}}
	feedItemRepo := &queryMockFeedItemRepo{items: []*aggregate.FeedItem{p1, p2, p3}}
	interactionRepo := &queryMockInteractionRepo{}
	userSnapshotRepo := &mockUserSnapshotRepo{}

	bloomFilter := &mockBloomFilterForQuery{
		seenMap: map[string]bool{
			"p-1": true, // p-1 has already been seen
		},
	}

	handler := NewGetActivityFeedHandler(
		followRepo,
		feedItemRepo,
		interactionRepo,
		userSnapshotRepo,
		WithFeedBloomFilter(bloomFilter),
	)

	// Refresh call (cursor = "")
	res, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// p-1 must be filtered out, leaving p-2 and p-3
	if len(res.Items) != 2 {
		t.Fatalf("expected 2 unseen items, got %d", len(res.Items))
	}
	if res.Items[0].ID != "p-2" || res.Items[1].ID != "p-3" {
		t.Errorf("expected [p-2, p-3], got [%s, %s]", res.Items[0].ID, res.Items[1].ID)
	}

	// Verify that returned items were added to seen
	if !bloomFilter.seenMap["p-2"] || !bloomFilter.seenMap["p-3"] {
		t.Errorf("expected returned items to be recorded as seen in bloom filter")
	}
}

func TestGetActivityFeedHandler_TrendingPoolBlending(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := aggregate.NewPostItem("p-1", "user-1", "Post 1", nil, vo.VisibilityPublic, 0, 0, now, now)
	hot1, _ := aggregate.NewPostItem("hot-1", "user-popular", "Trending 1", nil, vo.VisibilityPublic, 100, 50, now, now)
	hot2, _ := aggregate.NewPostItem("hot-2", "user-popular", "Trending 2", nil, vo.VisibilityPublic, 90, 40, now, now)

	followRepo := &queryMockFollowRepo{followingIDs: []string{"user-1"}}
	feedItemRepo := &queryMockFeedItemRepo{
		items: []*aggregate.FeedItem{p1, hot1, hot2},
	}
	interactionRepo := &queryMockInteractionRepo{}
	userSnapshotRepo := &mockUserSnapshotRepo{}

	bloomFilter := &mockBloomFilterForQuery{
		seenMap: make(map[string]bool),
	}
	trendingCache := &mockTrendingPoolCacheForQuery{
		trendingPool: []string{"hot-1", "hot-2"},
	}

	handler := NewGetActivityFeedHandler(
		followRepo,
		feedItemRepo,
		interactionRepo,
		userSnapshotRepo,
		WithFeedBloomFilter(bloomFilter),
		WithTrendingPoolCache(trendingCache),
	)

	// Request 3 items: following has only 1, so hot-1 and hot-2 should be blended
	res, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Items) != 3 {
		t.Fatalf("expected 3 items (1 following + 2 trending), got %d", len(res.Items))
	}
	if res.Items[0].ID != "p-1" || res.Items[1].ID != "hot-1" || res.Items[2].ID != "hot-2" {
		t.Errorf("expected [p-1, hot-1, hot-2], got [%s, %s, %s]",
			res.Items[0].ID, res.Items[1].ID, res.Items[2].ID)
	}
}

func TestGetActivityFeedHandler_80_20_RatioBlending(t *testing.T) {
	now := time.Now().UTC()
	var followingItems []*aggregate.FeedItem
	var followingIDs []string
	for i := 1; i <= 10; i++ {
		fID := fmt.Sprintf("friend-%d", i)
		followingIDs = append(followingIDs, fID)
		p, _ := aggregate.NewPostItem(fmt.Sprintf("f-%d", i), fID, "Post", nil, vo.VisibilityPublic, 0, 0, now.Add(time.Duration(i)*time.Minute), now)
		followingItems = append(followingItems, p)
	}

	hot1, _ := aggregate.NewPostItem("hot-1", "user-hot", "Hot 1", nil, vo.VisibilityPublic, 100, 50, now, now)
	hot2, _ := aggregate.NewPostItem("hot-2", "user-hot", "Hot 2", nil, vo.VisibilityPublic, 90, 40, now, now)

	allItems := make([]*aggregate.FeedItem, 0, len(followingItems)+2)
	allItems = append(allItems, followingItems...)
	allItems = append(allItems, hot1, hot2)

	followRepo := &queryMockFollowRepo{followingIDs: followingIDs}
	feedItemRepo := &queryMockFeedItemRepo{
		items: allItems,
	}
	interactionRepo := &queryMockInteractionRepo{}
	userSnapshotRepo := &mockUserSnapshotRepo{}

	bloomFilter := &mockBloomFilterForQuery{seenMap: make(map[string]bool)}
	trendingCache := &mockTrendingPoolCacheForQuery{trendingPool: []string{"hot-1", "hot-2"}}

	handler := NewGetActivityFeedHandler(
		followRepo,
		feedItemRepo,
		interactionRepo,
		userSnapshotRepo,
		WithFeedBloomFilter(bloomFilter),
		WithTrendingPoolCache(trendingCache),
	)

	// PageSize = 10 -> 80% following (8 items) + 20% trending (2 items)
	res, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Items) != 10 {
		t.Fatalf("expected exactly 10 items, got %d", len(res.Items))
	}

	followingCount := 0
	trendingCount := 0
	for _, it := range res.Items {
		if it.ID == "hot-1" || it.ID == "hot-2" {
			trendingCount++
		} else {
			followingCount++
		}
	}

	if followingCount != 8 || trendingCount != 2 {
		t.Fatalf("expected 8 following and 2 trending (80/20 ratio), got %d following and %d trending", followingCount, trendingCount)
	}
}

func TestGetActivityFeedHandler_CursorDoesNotSkewOnTrending(t *testing.T) {
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)

	// Friend has 12 posts today (posted every 10 mins descending)
	var friendPosts []*aggregate.FeedItem
	for i := 1; i <= 12; i++ {
		p, _ := aggregate.NewPostItem(
			fmt.Sprintf("f-%d", i),
			"user-friend",
			fmt.Sprintf("Friend post %d", i),
			nil,
			vo.VisibilityPublic,
			0, 0,
			now.Add(-time.Duration(i*10)*time.Minute),
			now.Add(-time.Duration(i*10)*time.Minute),
		)
		friendPosts = append(friendPosts, p)
	}

	// 2 Trending posts from YESTERDAY
	hot1, _ := aggregate.NewPostItem("hot-1", "user-popular", "Trending Yesterday 1", nil, vo.VisibilityPublic, 100, 50, yesterday, yesterday)
	hot2, _ := aggregate.NewPostItem("hot-2", "user-popular", "Trending Yesterday 2", nil, vo.VisibilityPublic, 90, 40, yesterday.Add(-time.Hour), yesterday.Add(-time.Hour))

	allItems := make([]*aggregate.FeedItem, 0, len(friendPosts)+2)
	allItems = append(allItems, friendPosts...)
	allItems = append(allItems, hot1, hot2)

	followRepo := &queryMockFollowRepo{followingIDs: []string{"user-friend"}}
	feedItemRepo := &queryMockFeedItemRepo{
		items: allItems,
	}
	interactionRepo := &queryMockInteractionRepo{}
	userSnapshotRepo := &mockUserSnapshotRepo{}

	bloomFilter := &mockBloomFilterForQuery{seenMap: make(map[string]bool)}
	trendingCache := &mockTrendingPoolCacheForQuery{trendingPool: []string{"hot-1", "hot-2"}}

	handler := NewGetActivityFeedHandler(
		followRepo,
		feedItemRepo,
		interactionRepo,
		userSnapshotRepo,
		WithFeedBloomFilter(bloomFilter),
		WithTrendingPoolCache(trendingCache),
	)

	// Page 1: Request 10 items
	// Expect 8 friend posts (f-1 .. f-8) + 2 trending posts (hot-1, hot-2)
	res1, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("page 1 unexpected error: %v", err)
	}
	if len(res1.Items) != 10 {
		t.Fatalf("expected 10 items on page 1, got %d", len(res1.Items))
	}

	// The last item in res1 is a trending post from yesterday!
	// But NextCursor MUST be the timestamp of f-8 (friend post today), NOT yesterday!
	expectedCursor := friendPosts[7].CreatedAt().Format("2006-01-02T15:04:05.000000Z07:00")
	if res1.NextCursor != expectedCursor {
		t.Fatalf("expected NextCursor to be friend post f-8 (%s), but got %s", expectedCursor, res1.NextCursor)
	}

	// Page 2: Request next 10 items using the cursor
	// Since cursor tracked f-8, page 2 MUST fetch f-9, f-10, f-11, f-12 from today!
	res2, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      10,
		Cursor:        res1.NextCursor,
	})
	if err != nil {
		t.Fatalf("page 2 unexpected error: %v", err)
	}

	// Verify that friend post f-9 is present in page 2 and was not skipped!
	foundF9 := false
	for _, it := range res2.Items {
		if it.ID == "f-9" {
			foundF9 = true
			break
		}
	}
	if !foundF9 {
		t.Errorf("friend post f-9 was skipped because cursor was skewed by trending post!")
	}
}

func TestGetActivityFeedHandler_SafetyValveEmergencyRecycle(t *testing.T) {
	now := time.Now().UTC()
	p1, _ := aggregate.NewPostItem("p-1", "user-1", "Post 1", nil, vo.VisibilityPublic, 10, 5, now, now)
	p2, _ := aggregate.NewPostItem("p-2", "user-2", "Post 2", nil, vo.VisibilityPublic, 20, 8, now.Add(-time.Hour), now.Add(-time.Hour))

	followRepo := &queryMockFollowRepo{followingIDs: []string{"user-1"}}
	feedItemRepo := &queryMockFeedItemRepo{
		items: []*aggregate.FeedItem{p1, p2},
	}
	interactionRepo := &queryMockInteractionRepo{}
	userSnapshotRepo := &mockUserSnapshotRepo{}

	// User has seen ALL posts on the platform!
	bloomFilter := &mockBloomFilterForQuery{
		seenMap: map[string]bool{
			"p-1": true,
			"p-2": true,
		},
	}
	trendingCache := &mockTrendingPoolCacheForQuery{
		trendingPool: []string{"p-1", "p-2"},
	}

	handler := NewGetActivityFeedHandler(
		followRepo,
		feedItemRepo,
		interactionRepo,
		userSnapshotRepo,
		WithFeedBloomFilter(bloomFilter),
		WithTrendingPoolCache(trendingCache),
	)

	// User refreshes feed after having seen 100% of posts
	res, err := handler.Handle(context.Background(), GetActivityFeedQuery{
		CurrentUserID: "user-me",
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Safety Valve must fire and return emergency public items instead of 0 items
	if len(res.Items) == 0 {
		t.Fatalf("expected safety valve to prevent empty screen, but got 0 items")
	}

	if len(res.Items) != 2 {
		t.Errorf("expected 2 emergency recycled items, got %d", len(res.Items))
	}
}
