package grpc_test

import (
	"context"
	"strings"
	"testing"
	"time"

	socialv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/query"
	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
	socialGRPC "github.com/viethung213/gym-companion/internal/social/transport/grpc"
)

type mockTx struct{}

func (m *mockTx) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockPublisher struct{}

func (m *mockPublisher) RecordEvents(ctx context.Context, events []any) error {
	return nil
}

func (m *mockPublisher) PublishEvents(ctx context.Context, events []any) error {
	return m.RecordEvents(ctx, events)
}

type testFollowRepo struct {
	follows map[string]*aggregate.Follow
}

func (r *testFollowRepo) Follow(ctx context.Context, f *aggregate.Follow) error {
	r.follows[f.FollowerID()+":"+f.FollowingID()] = f
	return nil
}
func (r *testFollowRepo) Unfollow(ctx context.Context, followerID, followingID string) error {
	delete(r.follows, followerID+":"+followingID)
	return nil
}
func (r *testFollowRepo) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	_, ok := r.follows[followerID+":"+followingID]
	return ok, nil
}
func (r *testFollowRepo) GetFollowers(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.Follow, string, int32, error) {
	return nil, "", 0, nil
}
func (r *testFollowRepo) GetFollowing(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.Follow, string, int32, error) {
	return nil, "", 0, nil
}
func (r *testFollowRepo) GetFollowingIDs(ctx context.Context, followerID string) ([]string, error) {
	return nil, nil
}
func (r *testFollowRepo) FilterFollowingIDs(ctx context.Context, followerID string, targetUserIDs []string) (map[string]bool, error) {
	res := make(map[string]bool)
	for _, targetID := range targetUserIDs {
		if r.follows[followerID+":"+targetID] != nil {
			res[targetID] = true
		}
	}
	return res, nil
}

type testFeedItemRepo struct {
	items map[string]*aggregate.FeedItem
}

func (r *testFeedItemRepo) Create(ctx context.Context, item *aggregate.FeedItem) error {
	r.items[item.ID()] = item
	return nil
}
func (r *testFeedItemRepo) GetByID(ctx context.Context, id string) (*aggregate.FeedItem, error) {
	return r.items[id], nil
}

func (r *testFeedItemRepo) GetByIDs(ctx context.Context, ids []string) ([]*aggregate.FeedItem, error) {
	var list []*aggregate.FeedItem
	for _, id := range ids {
		if it, ok := r.items[id]; ok {
			list = append(list, it)
		}
	}
	return list, nil
}

func (r *testFeedItemRepo) GetTrendingCandidateIDs(ctx context.Context, limit, hours int) ([]string, error) {
	ids := make([]string, 0, len(r.items))
	for id := range r.items {
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *testFeedItemRepo) GetPublicFeedItems(ctx context.Context, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	list := make([]*aggregate.FeedItem, 0, len(r.items))
	for _, it := range r.items {
		list = append(list, it)
	}
	return list, "", nil
}
func (r *testFeedItemRepo) Delete(ctx context.Context, id, userID string) error {
	delete(r.items, id)
	return nil
}
func (r *testFeedItemRepo) GetByUserID(ctx context.Context, userID string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	var list []*aggregate.FeedItem
	for _, item := range r.items {
		if item.UserID() == userID {
			list = append(list, item)
		}
	}
	return list, "", nil
}
func (r *testFeedItemRepo) GetByAuthorIDs(ctx context.Context, authorIDs []string, limit int, cursor string) ([]*aggregate.FeedItem, string, error) {
	return nil, "", nil
}
func (r *testFeedItemRepo) CountByUserID(ctx context.Context, userID string) (int32, error) {
	var count int32
	for _, item := range r.items {
		if item.UserID() == userID {
			count++
		}
	}
	return count, nil
}
func (r *testFeedItemRepo) UpdateReactionCount(ctx context.Context, id string, delta int) error {
	return nil
}
func (r *testFeedItemRepo) UpdateCommentCount(ctx context.Context, id string, delta int) error {
	return nil
}
func (r *testFeedItemRepo) SearchPosts(ctx context.Context, filter repository.SearchPostsFilter) ([]*aggregate.FeedItem, string, int32, error) {
	list := make([]*aggregate.FeedItem, 0, len(r.items))
	for _, item := range r.items {
		if filter.AuthorID != "" && item.UserID() != filter.AuthorID {
			continue
		}
		if filter.Query != "" && !strings.Contains(strings.ToLower(item.Caption()), strings.ToLower(filter.Query)) {
			continue
		}
		list = append(list, item)
	}
	return list, "", int32(len(list)), nil
}

type testInteractionRepo struct {
	reactions map[string]*entity.Reaction
	comments  map[string]*entity.Comment
}

func (r *testInteractionRepo) SaveReaction(ctx context.Context, reaction *entity.Reaction) error {
	r.reactions[reaction.UserID()+":"+reaction.FeedItemID()] = reaction
	return nil
}
func (r *testInteractionRepo) DeleteReaction(ctx context.Context, userID, feedItemID string) error {
	delete(r.reactions, userID+":"+feedItemID)
	return nil
}
func (r *testInteractionRepo) GetReaction(ctx context.Context, userID, feedItemID string) (*entity.Reaction, error) {
	return r.reactions[userID+":"+feedItemID], nil
}
func (r *testInteractionRepo) GetReactionsByFeedItem(ctx context.Context, feedItemID string) ([]*entity.Reaction, error) {
	var res []*entity.Reaction
	for _, reaction := range r.reactions {
		if reaction.FeedItemID() == feedItemID {
			res = append(res, reaction)
		}
	}
	return res, nil
}
func (r *testInteractionRepo) GetUserReactions(ctx context.Context, userID string, feedItemIDs []string) (map[string]*entity.Reaction, error) {
	res := make(map[string]*entity.Reaction)
	for _, fid := range feedItemIDs {
		if reaction, ok := r.reactions[userID+":"+fid]; ok {
			res[fid] = reaction
		}
	}
	return res, nil
}
func (r *testInteractionRepo) AddComment(ctx context.Context, comment *entity.Comment) error {
	r.comments[comment.ID()] = comment
	return nil
}
func (r *testInteractionRepo) GetCommentByID(ctx context.Context, id string) (*entity.Comment, error) {
	return r.comments[id], nil
}
func (r *testInteractionRepo) DeleteComment(ctx context.Context, id, userID string) error {
	delete(r.comments, id)
	return nil
}
func (r *testInteractionRepo) ListComments(ctx context.Context, feedItemID string, limit int, cursor string) ([]*entity.Comment, string, int32, error) {
	var list []*entity.Comment
	for _, c := range r.comments {
		if c.FeedItemID() == feedItemID {
			list = append(list, c)
		}
	}
	return list, "", int32(len(list)), nil
}

type testUserSnapshotRepo struct{}

func (p *testUserSnapshotRepo) GetByID(ctx context.Context, id string) (*entity.UserSnapshot, error) {
	return entity.NewUserSnapshotWithCounts(id, "Test User", "", "user", 1, 2, time.Time{}), nil
}
func (p *testUserSnapshotRepo) GetByIDs(ctx context.Context, ids []string) (map[string]*entity.UserSnapshot, error) {
	return nil, nil
}
func (p *testUserSnapshotRepo) Upsert(ctx context.Context, user *entity.UserSnapshot) error {
	return nil
}
func (p *testUserSnapshotRepo) UpdateRole(ctx context.Context, userID, newRole string) error {
	return nil
}
func (p *testUserSnapshotRepo) UpdateFollowCounts(ctx context.Context, followerID, followingID string, delta int) error {
	return nil
}
func (p *testUserSnapshotRepo) SearchUsers(ctx context.Context, q string, role string, limit int, cursor string) ([]*entity.UserSnapshot, string, int32, error) {
	return []*entity.UserSnapshot{
		entity.NewUserSnapshot("u-searched-1", "Searched User", "https://avatar.png", "user", time.Now()),
	}, "", 1, nil
}

func setupTestHandler() *socialGRPC.GRPCHandler {
	fRepo := &testFollowRepo{follows: make(map[string]*aggregate.Follow)}
	feedRepo := &testFeedItemRepo{items: make(map[string]*aggregate.FeedItem)}
	iRepo := &testInteractionRepo{reactions: make(map[string]*entity.Reaction), comments: make(map[string]*entity.Comment)}
	uRepo := &testUserSnapshotRepo{}
	tx := &mockTx{}
	pub := &mockPublisher{}

	return socialGRPC.NewGRPCHandler(
		command.NewFollowUserHandler(fRepo, uRepo, pub, tx),
		command.NewUnfollowUserHandler(fRepo, uRepo, tx),
		command.NewCreatePostHandler(feedRepo, pub, tx),
		command.NewDeleteFeedItemHandler(feedRepo, tx),
		command.NewReactTargetHandler(iRepo, feedRepo, uRepo, pub, tx),
		command.NewAddCommentHandler(iRepo, feedRepo, uRepo, pub, tx),
		command.NewDeleteCommentHandler(iRepo, feedRepo, tx),
		query.NewGetActivityFeedHandler(fRepo, feedRepo, iRepo, uRepo),
		query.NewGetUserProfileFeedHandler(feedRepo, uRepo),
		query.NewGetSocialSummaryHandler(fRepo, feedRepo, uRepo),
		query.NewGetFollowersHandler(fRepo, uRepo),
		query.NewGetFollowingHandler(fRepo, uRepo),
		query.NewListCommentsHandler(iRepo, uRepo),
		query.NewListReactionsHandler(iRepo, uRepo),
		query.NewSearchUsersHandler(uRepo, fRepo),
		query.NewSearchPostsHandler(feedRepo, iRepo, uRepo),
	)
}

func contextWithUser(userID string) context.Context {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, userID)
	return context.WithValue(ctx, middleware.UserRoleKey, "USER")
}

func TestGRPCHandler_FollowAndPost(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	// 1. Follow User
	followReq := &socialv1message.FollowUserRequest{
		FollowingId: "user-2",
	}
	followRes, err := h.FollowUser(authCtx, followReq)
	if err != nil {
		t.Fatalf("FollowUser failed: %v", err)
	}
	if !followRes.GetSuccess() {
		t.Errorf("expected follow success")
	}

	// 2. Create Post
	postReq := &socialv1message.CreatePostRequest{
		Caption:    "Hello gym buddies!",
		Visibility: "PUBLIC",
	}
	postRes, err := h.CreatePost(authCtx, postReq)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	if postRes.GetItem().GetCaption() != "Hello gym buddies!" {
		t.Errorf("unexpected post caption")
	}

	// 3. React to post
	reactReq := &socialv1message.ReactTargetRequest{
		FeedItemId:   postRes.GetItem().GetId(),
		ReactionType: socialv1message.ReactionType_REACTION_TYPE_FIRE,
	}
	reactRes, err := h.ReactTarget(authCtx, reactReq)
	if err != nil {
		t.Fatalf("ReactTarget failed: %v", err)
	}
	if reactRes.GetCurrentReaction() != "FIRE" {
		t.Errorf("expected FIRE reaction, got %s", reactRes.GetCurrentReaction())
	}

	// 4. Get Social Summary
	summaryReq := &socialv1message.GetSocialSummaryRequest{
		UserId: "user-1",
	}
	summaryRes, err := h.GetSocialSummary(authCtx, summaryReq)
	if err != nil {
		t.Fatalf("GetSocialSummary failed: %v", err)
	}
	if summaryRes.GetPostCount() != 1 {
		t.Errorf("expected 1 post, got %d", summaryRes.GetPostCount())
	}

	// 5. Delete Feed Item
	deleteReq := &socialv1message.DeleteFeedItemRequest{
		FeedItemId: postRes.GetItem().GetId(),
	}
	delRes, err := h.DeleteFeedItem(authCtx, deleteReq)
	if err != nil {
		t.Fatalf("DeleteFeedItem failed: %v", err)
	}
	if !delRes.GetSuccess() {
		t.Errorf("expected delete success")
	}
}

func TestGRPCHandler_UnauthenticatedRejection(t *testing.T) {
	h := setupTestHandler()
	unauthCtx := context.Background()

	// Calling FollowUser without auth should fail
	_, err := h.FollowUser(unauthCtx, &socialv1message.FollowUserRequest{FollowingId: "user-2"})
	if err == nil {
		t.Errorf("expected unauthenticated error")
	}
}

func TestGRPCHandler_MeEndpoints(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	// 1. GetSocialSummary without userId (calls /me/summary)
	summaryRes, err := h.GetSocialSummary(authCtx, &socialv1message.GetSocialSummaryRequest{})
	if err != nil {
		t.Fatalf("GetSocialSummary without userId failed: %v", err)
	}
	if got, want := summaryRes.GetUserId(), "user-1"; got != want {
		t.Errorf("expected userId %s, got %s", want, got)
	}
	if got, want := summaryRes.GetFullName(), "Test User"; got != want {
		t.Errorf("expected fullName %s, got %s", want, got)
	}
	if summaryRes.GetAvatarUrl() == "" {
		t.Errorf("expected non-empty avatarUrl")
	}
	if got, want := summaryRes.GetRole(), "user"; got != want {
		t.Errorf("expected role %s, got %s", want, got)
	}

	// 2. GetFollowers without userId (calls /me/followers)
	followersRes, err := h.GetFollowers(authCtx, &socialv1message.GetFollowersRequest{})
	if err != nil {
		t.Fatalf("GetFollowers without userId failed: %v", err)
	}
	if followersRes == nil {
		t.Errorf("expected followers response")
	}

	// 3. GetFollowing without userId (calls /me/following)
	followingRes, err := h.GetFollowing(authCtx, &socialv1message.GetFollowingRequest{})
	if err != nil {
		t.Fatalf("GetFollowing without userId failed: %v", err)
	}
	if followingRes == nil {
		t.Errorf("expected following response")
	}

	// 4. GetUserProfileFeed without userId (calls /me/feed)
	feedRes, err := h.GetUserProfileFeed(authCtx, &socialv1message.GetUserProfileFeedRequest{})
	if err != nil {
		t.Fatalf("GetUserProfileFeed without userId failed: %v", err)
	}
	if feedRes == nil {
		t.Errorf("expected profile feed response")
	}
}

func TestGRPCHandler_ListReactions(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	// 1. Create Post first
	postRes, err := h.CreatePost(authCtx, &socialv1message.CreatePostRequest{
		Caption:    "Post with reactions",
		Visibility: "PUBLIC",
	})
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	postID := postRes.GetItem().GetId()

	// 2. React target
	_, err = h.ReactTarget(authCtx, &socialv1message.ReactTargetRequest{
		FeedItemId:   postID,
		ReactionType: socialv1message.ReactionType_REACTION_TYPE_FIRE,
	})
	if err != nil {
		t.Fatalf("ReactTarget failed: %v", err)
	}

	// 3. List reactions
	res, err := h.ListReactions(context.Background(), &socialv1message.ListReactionsRequest{
		FeedItemId: postID,
	})
	if err != nil {
		t.Fatalf("ListReactions failed: %v", err)
	}

	if len(res.GetReactions()) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(res.GetReactions()))
	}
	if res.GetReactions()[0].GetUserId() != "user-1" {
		t.Errorf("expected user-1, got %s", res.GetReactions()[0].GetUserId())
	}
	if res.GetReactions()[0].GetReactionType() != socialv1message.ReactionType_REACTION_TYPE_FIRE {
		t.Errorf("expected FIRE, got %v", res.GetReactions()[0].GetReactionType())
	}
}

func TestGRPCHandler_UnfollowUser(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	// Follow first
	_, err := h.FollowUser(authCtx, &socialv1message.FollowUserRequest{
		FollowingId: "user-2",
	})
	if err != nil {
		t.Fatalf("FollowUser failed: %v", err)
	}

	// Unfollow
	res, err := h.UnfollowUser(authCtx, &socialv1message.UnfollowUserRequest{
		FollowingId: "user-2",
	})
	if err != nil {
		t.Fatalf("UnfollowUser failed: %v", err)
	}
	if !res.GetSuccess() {
		t.Errorf("expected unfollow success true")
	}
}

func TestGRPCHandler_Comments(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	// 1. Create Post
	postRes, err := h.CreatePost(authCtx, &socialv1message.CreatePostRequest{
		Caption: "Comment test post",
	})
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	postID := postRes.GetItem().GetId()

	// 2. Add comment
	commentRes, err := h.AddComment(authCtx, &socialv1message.AddCommentRequest{
		FeedItemId: postID,
		Content:    "Nice lift!",
	})
	if err != nil {
		t.Fatalf("AddComment failed: %v", err)
	}
	if commentRes.GetComment().GetContent() != "Nice lift!" {
		t.Errorf("expected content 'Nice lift!', got '%s'", commentRes.GetComment().GetContent())
	}
	commentID := commentRes.GetComment().GetId()

	// 3. List comments
	listRes, err := h.ListComments(authCtx, &socialv1message.ListCommentsRequest{
		FeedItemId: postID,
		PageSize:   10,
	})
	if err != nil {
		t.Fatalf("ListComments failed: %v", err)
	}
	if len(listRes.GetComments()) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(listRes.GetComments()))
	}

	// 4. Delete comment
	delRes, err := h.DeleteComment(authCtx, &socialv1message.DeleteCommentRequest{
		CommentId: commentID,
	})
	if err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}
	if !delRes.GetSuccess() {
		t.Errorf("expected delete comment success true")
	}
}

func TestGRPCHandler_ActivityFeed(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	feedRes, err := h.GetActivityFeed(authCtx, &socialv1message.GetActivityFeedRequest{
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("GetActivityFeed failed: %v", err)
	}
	if feedRes == nil {
		t.Errorf("expected non-nil response")
	}
}

func TestGRPCHandler_DeleteFeedItem(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	// 1. Create Post
	postRes, err := h.CreatePost(authCtx, &socialv1message.CreatePostRequest{
		Caption: "Post to be deleted",
	})
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	postID := postRes.GetItem().GetId()

	// 2. Delete Feed Item
	delRes, err := h.DeleteFeedItem(authCtx, &socialv1message.DeleteFeedItemRequest{
		FeedItemId: postID,
	})
	if err != nil {
		t.Fatalf("DeleteFeedItem failed: %v", err)
	}
	if !delRes.GetSuccess() {
		t.Errorf("expected success true")
	}
}

func TestGRPCHandler_GetFollowing(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	// Follow user-2
	_, err := h.FollowUser(authCtx, &socialv1message.FollowUserRequest{
		FollowingId: "user-2",
	})
	if err != nil {
		t.Fatalf("FollowUser failed: %v", err)
	}

	// Get following with userId specified
	res, err := h.GetFollowing(authCtx, &socialv1message.GetFollowingRequest{
		UserId:   "user-1",
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("GetFollowing failed: %v", err)
	}
	if res == nil {
		t.Errorf("expected non-nil response")
	}
}

func TestGRPCHandler_UnauthenticatedCases(t *testing.T) {
	h := setupTestHandler()
	noAuthCtx := context.Background()

	// FollowUser unauthenticated
	if _, err := h.FollowUser(noAuthCtx, &socialv1message.FollowUserRequest{}); err == nil {
		t.Errorf("expected unauthenticated error for FollowUser")
	}

	// UnfollowUser unauthenticated
	if _, err := h.UnfollowUser(noAuthCtx, &socialv1message.UnfollowUserRequest{}); err == nil {
		t.Errorf("expected unauthenticated error for UnfollowUser")
	}

	// CreatePost unauthenticated
	if _, err := h.CreatePost(noAuthCtx, &socialv1message.CreatePostRequest{}); err == nil {
		t.Errorf("expected unauthenticated error for CreatePost")
	}

	// DeleteFeedItem unauthenticated
	if _, err := h.DeleteFeedItem(noAuthCtx, &socialv1message.DeleteFeedItemRequest{}); err == nil {
		t.Errorf("expected unauthenticated error for DeleteFeedItem")
	}

	// AddComment unauthenticated
	if _, err := h.AddComment(noAuthCtx, &socialv1message.AddCommentRequest{}); err == nil {
		t.Errorf("expected unauthenticated error for AddComment")
	}

	// DeleteComment unauthenticated
	if _, err := h.DeleteComment(noAuthCtx, &socialv1message.DeleteCommentRequest{}); err == nil {
		t.Errorf("expected unauthenticated error for DeleteComment")
	}

	// ReactTarget unauthenticated
	if _, err := h.ReactTarget(noAuthCtx, &socialv1message.ReactTargetRequest{}); err == nil {
		t.Errorf("expected unauthenticated error for ReactTarget")
	}

	// GetActivityFeed unauthenticated
	if _, err := h.GetActivityFeed(noAuthCtx, &socialv1message.GetActivityFeedRequest{}); err == nil {
		t.Errorf("expected unauthenticated error for GetActivityFeed")
	}
}

func TestGRPCHandler_SearchUsers(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	res, err := h.SearchUsers(authCtx, &socialv1message.SearchUsersRequest{
		Query:    "Searched",
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("SearchUsers failed: %v", err)
	}
	if len(res.GetUsers()) != 1 {
		t.Fatalf("expected 1 user, got %d", len(res.GetUsers()))
	}
	if res.GetUsers()[0].GetUserId() != "u-searched-1" {
		t.Errorf("expected user id u-searched-1, got %s", res.GetUsers()[0].GetUserId())
	}
}

func TestGRPCHandler_SearchPosts(t *testing.T) {
	h := setupTestHandler()
	authCtx := contextWithUser("user-1")

	// 1. Create a post
	_, err := h.CreatePost(authCtx, &socialv1message.CreatePostRequest{
		Caption:    "Testing squat search capability",
		Visibility: "PUBLIC",
	})
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	// 2. Search for squat
	res, err := h.SearchPosts(authCtx, &socialv1message.SearchPostsRequest{
		Query:    "squat",
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("SearchPosts failed: %v", err)
	}
	if len(res.GetItems()) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.GetItems()))
	}
	if res.GetItems()[0].GetCaption() != "Testing squat search capability" {
		t.Errorf("caption mismatch: %s", res.GetItems()[0].GetCaption())
	}
}
