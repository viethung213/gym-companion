package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
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
