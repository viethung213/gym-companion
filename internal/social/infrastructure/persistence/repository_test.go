package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
	"github.com/viethung213/gym-companion/internal/social/test/testutil"
)

func TestRepositories_FollowAndFeed(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Follow Repository
	f, _ := aggregate.NewFollow("f-1", "user-1", "user-2", now)
	if err := repos.FollowRepo.Follow(ctx, f); err != nil {
		t.Fatalf("Follow failed: %v", err)
	}

	isFollow, err := repos.FollowRepo.IsFollowing(ctx, "user-1", "user-2")
	if err != nil || !isFollow {
		t.Fatalf("expected isFollowing true, got %v, err: %v", isFollow, err)
	}

	followersCount, _ := repos.FollowRepo.CountFollowers(ctx, "user-2")
	if followersCount != 1 {
		t.Fatalf("expected 1 follower, got %d", followersCount)
	}

	followingCount, _ := repos.FollowRepo.CountFollowing(ctx, "user-1")
	if followingCount != 1 {
		t.Fatalf("expected 1 following, got %d", followingCount)
	}

	followingIDs, _ := repos.FollowRepo.GetFollowingIDs(ctx, "user-1")
	if len(followingIDs) != 1 || followingIDs[0] != "user-2" {
		t.Fatalf("expected [user-2], got %v", followingIDs)
	}

	filtered, err := repos.FollowRepo.FilterFollowingIDs(ctx, "user-1", []string{"user-2", "user-non-existent"})
	if err != nil || !filtered["user-2"] || filtered["user-non-existent"] {
		t.Fatalf("FilterFollowingIDs failed: err=%v, filtered=%v", err, filtered)
	}

	batchFollowersCount, err := repos.FollowRepo.CountFollowersByIDs(ctx, []string{"user-2", "user-non-existent"})
	if err != nil || batchFollowersCount["user-2"] != 1 {
		t.Fatalf("CountFollowersByIDs failed: err=%v, counts=%v", err, batchFollowersCount)
	}

	// 2. Feed Item Repository
	post, _ := aggregate.NewPostItem("p-1", "user-2", "Hello World", nil, vo.VisibilityPublic, 0, 0, now, now)
	if err := repos.FeedItemRepo.Create(ctx, post); err != nil {
		t.Fatalf("FeedItem Create failed: %v", err)
	}

	item, err := repos.FeedItemRepo.GetByID(ctx, "p-1")
	if err != nil || item == nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if item.Caption() != "Hello World" {
		t.Fatalf("expected 'Hello World', got %s", item.Caption())
	}

	_ = repos.FeedItemRepo.UpdateReactionCount(ctx, "p-1", 1)
	_ = repos.FeedItemRepo.UpdateCommentCount(ctx, "p-1", 1)

	// Feed By Author IDs
	items, _, err := repos.FeedItemRepo.GetByAuthorIDs(ctx, []string{"user-2"}, 10, "")
	if err != nil || len(items) != 1 {
		t.Fatalf("GetByAuthorIDs failed: items=%d, err=%v", len(items), err)
	}

	// 3. Unfollow
	if err := repos.FollowRepo.Unfollow(ctx, "user-1", "user-2"); err != nil {
		t.Fatalf("Unfollow failed: %v", err)
	}
	isFollow, _ = repos.FollowRepo.IsFollowing(ctx, "user-1", "user-2")
	if isFollow {
		t.Fatalf("expected isFollowing false after unfollow")
	}

	// 4. Delete Feed Item
	if err := repos.FeedItemRepo.Delete(ctx, "p-1", "user-2"); err != nil {
		t.Fatalf("FeedItem Delete failed: %v", err)
	}
}

func TestRepositories_InteractionsAndUser(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. User Snapshot
	u := entity.NewUserSnapshot("u-test-1", "Test User", "https://avatar.png", "user", now)
	if err := repos.UserSnapshotRepo.Upsert(ctx, u); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	fetchedUser, err := repos.UserSnapshotRepo.GetByID(ctx, "u-test-1")
	if err != nil || fetchedUser == nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	userMap, err := repos.UserSnapshotRepo.GetByIDs(ctx, []string{"u-test-1", "u-non-existent"})
	if err != nil || len(userMap) != 2 {
		t.Fatalf("GetByIDs failed: %v", err)
	}

	// Update role to brand
	if err := repos.UserSnapshotRepo.UpdateRole(ctx, "u-test-1", "brand"); err != nil {
		t.Fatalf("UpdateRole failed: %v", err)
	}
	updatedUser, _ := repos.UserSnapshotRepo.GetByID(ctx, "u-test-1")
	if updatedUser.Role() != "brand" {
		t.Fatalf("expected role brand, got %s", updatedUser.Role())
	}

	// Search Users
	searched, _, total, err := repos.UserSnapshotRepo.SearchUsers(ctx, "Test", "brand", 10, "")
	if err != nil || len(searched) != 1 || total != 1 {
		t.Fatalf("SearchUsers failed: len=%d, total=%d, err=%v", len(searched), total, err)
	}

	// Update Follow Counts
	u2 := entity.NewUserSnapshot("u-test-2", "Followed User", "https://avatar2.png", "user", now)
	if err := repos.UserSnapshotRepo.Upsert(ctx, u2); err != nil {
		t.Fatalf("Upsert u2 failed: %v", err)
	}
	if err := repos.UserSnapshotRepo.UpdateFollowCounts(ctx, "u-test-1", "u-test-2", 1); err != nil {
		t.Fatalf("UpdateFollowCounts +1 failed: %v", err)
	}
	u1Fetched, _ := repos.UserSnapshotRepo.GetByID(ctx, "u-test-1")
	u2Fetched, _ := repos.UserSnapshotRepo.GetByID(ctx, "u-test-2")
	if u1Fetched.FollowingCount() != 1 || u2Fetched.FollowerCount() != 1 {
		t.Fatalf("expected u1 following=1, u2 follower=1; got following=%d, follower=%d",
			u1Fetched.FollowingCount(), u2Fetched.FollowerCount())
	}
	// Decrement counts
	if err := repos.UserSnapshotRepo.UpdateFollowCounts(ctx, "u-test-1", "u-test-2", -1); err != nil {
		t.Fatalf("UpdateFollowCounts -1 failed: %v", err)
	}
	u1Fetched, _ = repos.UserSnapshotRepo.GetByID(ctx, "u-test-1")
	u2Fetched, _ = repos.UserSnapshotRepo.GetByID(ctx, "u-test-2")
	if u1Fetched.FollowingCount() != 0 || u2Fetched.FollowerCount() != 0 {
		t.Fatalf("expected counts clamped to 0; got following=%d, follower=%d",
			u1Fetched.FollowingCount(), u2Fetched.FollowerCount())
	}
	// Decrement again to verify clamp at 0
	if err := repos.UserSnapshotRepo.UpdateFollowCounts(ctx, "u-test-1", "u-test-2", -1); err != nil {
		t.Fatalf("UpdateFollowCounts clamp failed: %v", err)
	}
	u1Fetched, _ = repos.UserSnapshotRepo.GetByID(ctx, "u-test-1")
	u2Fetched, _ = repos.UserSnapshotRepo.GetByID(ctx, "u-test-2")
	if u1Fetched.FollowingCount() != 0 || u2Fetched.FollowerCount() != 0 {
		t.Fatalf("expected counts clamped to 0 after extra decrement; got following=%d, follower=%d",
			u1Fetched.FollowingCount(), u2Fetched.FollowerCount())
	}

	// 2. Interaction (Reactions & Comments)
	r, _ := entity.NewReaction("r-1", "u-test-1", "target-1", vo.ReactionTypeFire, now)
	if err := repos.InteractionRepo.SaveReaction(ctx, r); err != nil {
		t.Fatalf("SaveReaction failed: %v", err)
	}
	existing, _ := repos.InteractionRepo.GetReaction(ctx, "u-test-1", "target-1")
	if existing == nil || existing.ReactionType() != vo.ReactionTypeFire {
		t.Fatalf("expected FIRE reaction")
	}
	reactions, err := repos.InteractionRepo.GetReactionsByFeedItem(ctx, "target-1")
	if err != nil || len(reactions) != 1 {
		t.Fatalf("GetReactionsByFeedItem failed: %v", err)
	}

	batchReactions, err := repos.InteractionRepo.GetUserReactions(ctx, "u-test-1", []string{"target-1", "non-existent"})
	if err != nil || len(batchReactions) != 1 || batchReactions["target-1"] == nil || batchReactions["target-1"].ReactionType() != vo.ReactionTypeFire {
		t.Fatalf("GetUserReactions failed: %v, len=%d", err, len(batchReactions))
	}

	c, _ := entity.NewComment("c-1", "u-test-1", "target-1", nil, "Awesome!", now, now)
	if err := repos.InteractionRepo.AddComment(ctx, c); err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
	comments, _, count, err := repos.InteractionRepo.ListComments(ctx, "target-1", 10, "")
	if err != nil || len(comments) != 1 || count != 1 {
		t.Fatalf("ListComments failed: %v", err)
	}

	// Remove reaction & comment
	if err := repos.InteractionRepo.DeleteReaction(ctx, "u-test-1", "target-1"); err != nil {
		t.Fatalf("DeleteReaction failed: %v", err)
	}
	if err := repos.InteractionRepo.DeleteComment(ctx, "c-1", "u-test-1"); err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}
}

func TestRepositories_OutboxAndTxManager(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()

	// 1. Outbox
	record := &port.OutboxRecord{
		ID:           "out-1",
		EventID:      "evt-1",
		EventType:    "test.event",
		Payload:      []byte(`{"key":"value"}`),
		PartitionKey: "part-1",
	}
	if err := repos.OutboxRepo.Save(ctx, record); err != nil {
		t.Fatalf("Outbox Save failed: %v", err)
	}

	unpub, err := repos.OutboxRepo.FetchUnpublished(ctx, 10)
	if err != nil || len(unpub) != 1 {
		t.Fatalf("FetchUnpublished failed: %v", err)
	}

	claimed, err := repos.OutboxRepo.ClaimBatch(ctx, 10, 5*time.Second)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("ClaimBatch failed: %v", err)
	}

	if err := repos.OutboxRepo.MarkAsPublished(ctx, []string{"out-1"}); err != nil {
		t.Fatalf("MarkAsPublished failed: %v", err)
	}

	// 2. Outbox Log
	if err := repos.OutboxLogRepo.Save(ctx, "evt-1", "test.event", []byte(`{}`), "key", "PROCESSED", ""); err != nil {
		t.Fatalf("OutboxLog Save failed: %v", err)
	}
	isProcessed, err := repos.OutboxLogRepo.IsProcessed(ctx, "evt-1")
	if err != nil || !isProcessed {
		t.Fatalf("expected isProcessed true")
	}

	// 3. TxManager rollback
	err = repos.TxManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return errors.New("simulated rollback")
	})
	if err == nil {
		t.Fatalf("expected rollback error")
	}
}

func TestRepositories_SearchPosts(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// user-1 follows user-2
	f, _ := aggregate.NewFollow("f-10", "user-1", "user-2", now)
	_ = repos.FollowRepo.Follow(ctx, f)

	authorUUID3 := "00000000-0000-0000-0000-000000000003"

	// Create posts with various visibilities and captions
	// 1. user-2 public squat post
	p1, _ := aggregate.NewPostItem("sp-1", "user-2", "Heavy squat session 140kg", nil, vo.VisibilityPublic, 0, 0, now.Add(-3*time.Minute), now)
	_ = repos.FeedItemRepo.Create(ctx, p1)

	// 2. user-2 followers-only bench post
	p2, _ := aggregate.NewPostItem("sp-2", "user-2", "Bench press personal record", nil, vo.VisibilityFollowersOnly, 0, 0, now.Add(-2*time.Minute), now)
	_ = repos.FeedItemRepo.Create(ctx, p2)

	// 3. user-2 private deadlift post
	p3, _ := aggregate.NewPostItem("sp-3", "user-2", "Deadlift form check", nil, vo.VisibilityPrivate, 0, 0, now.Add(-1*time.Minute), now)
	_ = repos.FeedItemRepo.Create(ctx, p3)

	// 4. user-3 public squat post
	p4, _ := aggregate.NewPostItem("sp-4", authorUUID3, "Morning squat routine", nil, vo.VisibilityPublic, 0, 0, now, now)
	_ = repos.FeedItemRepo.Create(ctx, p4)

	t.Run("unauthenticated viewer only sees public posts", func(t *testing.T) {
		items, _, total, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "",
			Query:    "squat",
		})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if total != 2 || len(items) != 2 {
			t.Fatalf("expected 2 public squat posts, got total=%d, len=%d", total, len(items))
		}
	})

	t.Run("follower sees public and followers_only posts of followed user", func(t *testing.T) {
		items, _, total, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "user-1",
			Query:    "record",
		})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if total != 1 || len(items) != 1 {
			t.Fatalf("expected 1 post, got total=%d", total)
		}
		if items[0].ID() != "sp-2" {
			t.Errorf("expected sp-2, got %s", items[0].ID())
		}
	})

	t.Run("stranger cannot see followers_only or private posts", func(t *testing.T) {
		items, _, total, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "user-4",
			Query:    "record",
		})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if total != 0 || len(items) != 0 {
			t.Fatalf("expected 0 posts, got %d", total)
		}
	})

	t.Run("author can see own private posts in search", func(t *testing.T) {
		items, _, total, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "user-2",
			Query:    "deadlift",
		})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if total != 1 || len(items) != 1 {
			t.Fatalf("expected 1 post, got %d", total)
		}
		if items[0].ID() != "sp-3" {
			t.Errorf("expected sp-3, got %s", items[0].ID())
		}
	})

	t.Run("author_id filter limits results to specific author", func(t *testing.T) {
		items, _, total, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "user-1",
			AuthorID: authorUUID3,
			Query:    "squat",
		})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if total != 1 || len(items) != 1 {
			t.Fatalf("expected 1 post, got %d", total)
		}
		if items[0].UserID() != authorUUID3 {
			t.Errorf("expected post from user-3, got %s", items[0].UserID())
		}
	})

	t.Run("escapes special like characters percentage and underscore", func(t *testing.T) {
		pWild, _ := aggregate.NewPostItem("sp-wild", "user-2", "100% pure effort_record", nil, vo.VisibilityPublic, 0, 0, now, now)
		_ = repos.FeedItemRepo.Create(ctx, pWild)

		items, _, total, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "",
			Query:    "100%",
		})
		if err != nil {
			t.Fatalf("search with %% failed: %v", err)
		}
		if total != 1 || len(items) != 1 || items[0].ID() != "sp-wild" {
			t.Fatalf("expected 1 item with 100%%, got total=%d", total)
		}

		itemsUnderscore, _, totalU, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "",
			Query:    "effort_record",
		})
		if err != nil {
			t.Fatalf("search with _ failed: %v", err)
		}
		if totalU != 1 || len(itemsUnderscore) != 1 || itemsUnderscore[0].ID() != "sp-wild" {
			t.Fatalf("expected 1 item with effort_record, got total=%d", totalU)
		}
	})

	t.Run("compound cursor pagination works correctly", func(t *testing.T) {
		pPage1, nextCur, total, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "user-2",
			Limit:    2,
			Query:    "squat",
		})
		if err != nil {
			t.Fatalf("page 1 failed: %v", err)
		}
		if len(pPage1) != 2 || total != 2 {
			t.Fatalf("expected 2 items on page 1, got len=%d total=%d", len(pPage1), total)
		}
		if nextCur == "" {
			t.Fatalf("expected non-empty nextCursor")
		}

		pPage2, _, total2, err := repos.FeedItemRepo.SearchPosts(ctx, repository.SearchPostsFilter{
			ViewerID: "user-2",
			Limit:    2,
			Query:    "squat",
			Cursor:   nextCur,
		})
		if err != nil {
			t.Fatalf("page 2 failed: %v", err)
		}
		if len(pPage2) != 0 || total2 != 2 {
			t.Fatalf("expected 0 items on page 2, got %d", len(pPage2))
		}
	})
}
