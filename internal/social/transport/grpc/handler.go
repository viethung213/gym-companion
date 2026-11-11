package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"
	socialv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/service/socialv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/query"
	"github.com/viethung213/gym-companion/internal/social/domain/derror"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//nolint:revive // GRPCHandler name is consistent across bounded contexts
type GRPCHandler struct {
	followUserHandler         *command.FollowUserHandler
	unfollowUserHandler       *command.UnfollowUserHandler
	createPostHandler         *command.CreatePostHandler
	deleteFeedItemHandler     *command.DeleteFeedItemHandler
	reactTargetHandler        *command.ReactTargetHandler
	addCommentHandler         *command.AddCommentHandler
	deleteCommentHandler      *command.DeleteCommentHandler
	getActivityFeedHandler    *query.GetActivityFeedHandler
	getUserProfileFeedHandler *query.GetUserProfileFeedHandler
	getSocialSummaryHandler   *query.GetSocialSummaryHandler
	getFollowersHandler       *query.GetFollowersHandler
	getFollowingHandler       *query.GetFollowingHandler
	listCommentsHandler       *query.ListCommentsHandler
	listReactionsHandler      *query.ListReactionsHandler
	searchUsersHandler        *query.SearchUsersHandler
	searchPostsHandler        *query.SearchPostsHandler
}

var _ socialv1serviceconnect.SocialServiceHandler = (*GRPCHandler)(nil)

func NewGRPCHandler(
	followUserHandler *command.FollowUserHandler,
	unfollowUserHandler *command.UnfollowUserHandler,
	createPostHandler *command.CreatePostHandler,
	deleteFeedItemHandler *command.DeleteFeedItemHandler,
	reactTargetHandler *command.ReactTargetHandler,
	addCommentHandler *command.AddCommentHandler,
	deleteCommentHandler *command.DeleteCommentHandler,
	getActivityFeedHandler *query.GetActivityFeedHandler,
	getUserProfileFeedHandler *query.GetUserProfileFeedHandler,
	getSocialSummaryHandler *query.GetSocialSummaryHandler,
	getFollowersHandler *query.GetFollowersHandler,
	getFollowingHandler *query.GetFollowingHandler,
	listCommentsHandler *query.ListCommentsHandler,
	listReactionsHandler *query.ListReactionsHandler,
	searchUsersHandler *query.SearchUsersHandler,
	searchPostsHandler *query.SearchPostsHandler,
) *GRPCHandler {
	return &GRPCHandler{
		followUserHandler:         followUserHandler,
		unfollowUserHandler:       unfollowUserHandler,
		createPostHandler:         createPostHandler,
		deleteFeedItemHandler:     deleteFeedItemHandler,
		reactTargetHandler:        reactTargetHandler,
		addCommentHandler:         addCommentHandler,
		deleteCommentHandler:      deleteCommentHandler,
		getActivityFeedHandler:    getActivityFeedHandler,
		getUserProfileFeedHandler: getUserProfileFeedHandler,
		getSocialSummaryHandler:   getSocialSummaryHandler,
		getFollowersHandler:       getFollowersHandler,
		getFollowingHandler:       getFollowingHandler,
		listCommentsHandler:       listCommentsHandler,
		listReactionsHandler:      listReactionsHandler,
		searchUsersHandler:        searchUsersHandler,
		searchPostsHandler:        searchPostsHandler,
	}
}

func (h *GRPCHandler) FollowUser(
	ctx context.Context,
	req *socialv1message.FollowUserRequest,
) (*socialv1message.FollowUserResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "authentication required")
	}

	err = h.followUserHandler.Handle(ctx, command.FollowUserCommand{
		FollowerID:  actor.UserID,
		FollowingID: req.GetFollowingId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}

	return &socialv1message.FollowUserResponse{
		Success: true,
		Message: "Followed successfully",
	}, nil
}

func (h *GRPCHandler) UnfollowUser(
	ctx context.Context,
	req *socialv1message.UnfollowUserRequest,
) (*socialv1message.UnfollowUserResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "authentication required")
	}

	err = h.unfollowUserHandler.Handle(ctx, command.UnfollowUserCommand{
		FollowerID:  actor.UserID,
		FollowingID: req.GetFollowingId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}

	return &socialv1message.UnfollowUserResponse{
		Success: true,
		Message: "Unfollowed successfully",
	}, nil
}

func (h *GRPCHandler) GetFollowers(
	ctx context.Context,
	req *socialv1message.GetFollowersRequest,
) (*socialv1message.GetFollowersResponse, error) {
	targetUserID := req.GetUserId()
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}
	if targetUserID == "" {
		targetUserID = currentUserID
	}
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "user_id required or authentication required")
	}

	res, err := h.getFollowersHandler.Handle(ctx, query.GetFollowersQuery{
		UserID:        targetUserID,
		CurrentUserID: currentUserID,
		PageSize:      int(req.GetPageSize()),
		Cursor:        req.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	pbFollowers := make([]*socialv1message.UserSocialSummary, len(res.Followers))
	for i, f := range res.Followers {
		pbFollowers[i] = &socialv1message.UserSocialSummary{
			UserId:      f.UserID,
			FullName:    f.FullName,
			AvatarUrl:   f.AvatarURL,
			IsFollowing: f.IsFollowing,
		}
	}

	return &socialv1message.GetFollowersResponse{
		Followers:  pbFollowers,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}, nil
}

func (h *GRPCHandler) GetFollowing(
	ctx context.Context,
	req *socialv1message.GetFollowingRequest,
) (*socialv1message.GetFollowingResponse, error) {
	targetUserID := req.GetUserId()
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}
	if targetUserID == "" {
		targetUserID = currentUserID
	}
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "user_id required or authentication required")
	}

	res, err := h.getFollowingHandler.Handle(ctx, query.GetFollowingQuery{
		UserID:        targetUserID,
		CurrentUserID: currentUserID,
		PageSize:      int(req.GetPageSize()),
		Cursor:        req.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	pbFollowing := make([]*socialv1message.UserSocialSummary, len(res.Following))
	for i, f := range res.Following {
		pbFollowing[i] = &socialv1message.UserSocialSummary{
			UserId:      f.UserID,
			FullName:    f.FullName,
			AvatarUrl:   f.AvatarURL,
			IsFollowing: f.IsFollowing,
		}
	}

	return &socialv1message.GetFollowingResponse{
		Following:  pbFollowing,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}, nil
}

func (h *GRPCHandler) GetSocialSummary(
	ctx context.Context,
	req *socialv1message.GetSocialSummaryRequest,
) (*socialv1message.GetSocialSummaryResponse, error) {
	targetUserID := req.GetUserId()
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}
	if targetUserID == "" {
		targetUserID = currentUserID
	}
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "user_id required or authentication required")
	}

	res, err := h.getSocialSummaryHandler.Handle(ctx, query.GetSocialSummaryQuery{
		TargetUserID:  targetUserID,
		CurrentUserID: currentUserID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &socialv1message.GetSocialSummaryResponse{
		UserId:         res.UserID,
		FullName:       res.FullName,
		AvatarUrl:      res.AvatarURL,
		Role:           res.Role,
		FollowerCount:  res.FollowerCount,
		FollowingCount: res.FollowingCount,
		PostCount:      res.PostCount,
		IsFollowing:    res.IsFollowing,
	}, nil
}

func (h *GRPCHandler) SearchUsers(
	ctx context.Context,
	req *socialv1message.SearchUsersRequest,
) (*socialv1message.SearchUsersResponse, error) {
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}

	res, err := h.searchUsersHandler.Handle(ctx, query.SearchUsersQuery{
		CurrentUserID: currentUserID,
		Query:         req.GetQuery(),
		Role:          req.GetRole(),
		PageSize:      int(req.GetPageSize()),
		Cursor:        req.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	pbUsers := make([]*socialv1message.DiscoverUserItem, len(res.Users))
	for i, u := range res.Users {
		pbUsers[i] = &socialv1message.DiscoverUserItem{
			UserId:        u.UserID,
			FullName:      u.FullName,
			AvatarUrl:     u.AvatarURL,
			Role:          u.Role,
			IsFollowing:   u.IsFollowing,
			FollowerCount: u.FollowerCount,
		}
	}

	return &socialv1message.SearchUsersResponse{
		Users:      pbUsers,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}, nil
}

func (h *GRPCHandler) CreatePost(
	ctx context.Context,
	req *socialv1message.CreatePostRequest,
) (*socialv1message.CreatePostResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "authentication required")
	}

	res, err := h.createPostHandler.Handle(ctx, command.CreatePostCommand{
		UserID:     actor.UserID,
		Caption:    req.GetCaption(),
		MediaURLs:  req.GetMediaUrls(),
		Visibility: req.GetVisibility(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}

	item := res.Item
	return &socialv1message.CreatePostResponse{
		Item: &socialv1message.FeedItem{
			Id:            item.ID(),
			UserId:        item.UserID(),
			ItemType:      socialv1message.FeedItemType_FEED_ITEM_TYPE_POST,
			Caption:       item.Caption(),
			MediaUrls:     item.MediaURLs(),
			Visibility:    item.Visibility().String(),
			ReactionCount: item.ReactionCount(),
			CommentCount:  item.CommentCount(),
			UserReaction:  "",
			CreatedAt:     timestamppb.New(item.CreatedAt()),
			UpdatedAt:     timestamppb.New(item.UpdatedAt()),
		},
	}, nil
}

func (h *GRPCHandler) DeleteFeedItem(
	ctx context.Context,
	req *socialv1message.DeleteFeedItemRequest,
) (*socialv1message.DeleteFeedItemResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "authentication required")
	}

	err = h.deleteFeedItemHandler.Handle(ctx, command.DeleteFeedItemCommand{
		FeedItemID: req.GetFeedItemId(),
		UserID:     actor.UserID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &socialv1message.DeleteFeedItemResponse{Success: true}, nil
}

func mapFeedItemDTOToProto(item *query.FeedItemDTO) *socialv1message.FeedItem {
	itemType := socialv1message.FeedItemType_FEED_ITEM_TYPE_POST
	if item.ItemType == "WORKOUT_ACTIVITY" {
		itemType = socialv1message.FeedItemType_FEED_ITEM_TYPE_WORKOUT_ACTIVITY
	}

	pbItem := &socialv1message.FeedItem{
		Id:              item.ID,
		UserId:          item.UserID,
		AuthorName:      item.AuthorName,
		AuthorAvatarUrl: item.AuthorAvatarURL,
		ItemType:        itemType,
		Caption:         item.Caption,
		MediaUrls:       item.MediaURLs,
		Visibility:      item.Visibility,
		ReactionCount:   item.ReactionCount,
		CommentCount:    item.CommentCount,
		UserReaction:    item.UserReaction,
		CreatedAt:       timestamppb.New(item.CreatedAt),
		UpdatedAt:       timestamppb.New(item.UpdatedAt),
	}

	if item.ItemType == "WORKOUT_ACTIVITY" && !item.WorkoutData.IsZero() {
		pbItem.WorkoutData = &socialv1message.WorkoutMetricsSummary{
			SessionId:       item.WorkoutData.SessionID(),
			WorkoutTitle:    item.WorkoutData.WorkoutTitle(),
			DurationSeconds: item.WorkoutData.DurationSeconds(),
			TotalVolumeKg:   float32(item.WorkoutData.TotalVolumeKg()),
			ExerciseCount:   item.WorkoutData.ExerciseCount(),
			TotalSets:       item.WorkoutData.TotalSets(),
		}
	}

	return pbItem
}

func (h *GRPCHandler) GetActivityFeed(
	ctx context.Context,
	req *socialv1message.GetActivityFeedRequest,
) (*socialv1message.GetActivityFeedResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "authentication required")
	}

	res, err := h.getActivityFeedHandler.Handle(ctx, query.GetActivityFeedQuery{
		CurrentUserID: actor.UserID,
		PageSize:      int(req.GetPageSize()),
		Cursor:        req.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	pbItems := make([]*socialv1message.FeedItem, 0, len(res.Items))
	for _, item := range res.Items {
		pbItems = append(pbItems, mapFeedItemDTOToProto(item))
	}

	return &socialv1message.GetActivityFeedResponse{
		Items:      pbItems,
		NextCursor: res.NextCursor,
	}, nil
}

func (h *GRPCHandler) GetUserProfileFeed(
	ctx context.Context,
	req *socialv1message.GetUserProfileFeedRequest,
) (*socialv1message.GetUserProfileFeedResponse, error) {
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}

	targetUserID := req.GetUserId()
	if targetUserID == "" {
		targetUserID = currentUserID
	}
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "user_id required or authentication required")
	}

	res, err := h.getUserProfileFeedHandler.Handle(ctx, query.GetUserProfileFeedQuery{
		TargetUserID:  targetUserID,
		CurrentUserID: currentUserID,
		PageSize:      int(req.GetPageSize()),
		Cursor:        req.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	pbItems := make([]*socialv1message.FeedItem, 0, len(res.Items))
	for _, item := range res.Items {
		pbItems = append(pbItems, mapFeedItemDTOToProto(item))
	}

	return &socialv1message.GetUserProfileFeedResponse{
		Items:      pbItems,
		NextCursor: res.NextCursor,
	}, nil
}

func (h *GRPCHandler) SearchPosts(
	ctx context.Context,
	req *socialv1message.SearchPostsRequest,
) (*socialv1message.SearchPostsResponse, error) {
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}

	itemTypeStr := ""
	switch req.GetItemType() {
	case socialv1message.FeedItemType_FEED_ITEM_TYPE_POST:
		itemTypeStr = "POST"
	case socialv1message.FeedItemType_FEED_ITEM_TYPE_WORKOUT_ACTIVITY:
		itemTypeStr = "WORKOUT_ACTIVITY"
	}

	authorID := ""
	if req.UserId != nil {
		authorID = req.GetUserId()
	}

	res, err := h.searchPostsHandler.Handle(ctx, query.SearchPostsQuery{
		ViewerID: currentUserID,
		AuthorID: authorID,
		Query:    req.GetQuery(),
		ItemType: itemTypeStr,
		PageSize: int(req.GetPageSize()),
		Cursor:   req.GetCursor(),
	})
	if err != nil {
		if errors.Is(err, derror.ErrEmptySearchQuery) ||
			errors.Is(err, derror.ErrSearchQueryTooShort) ||
			errors.Is(err, derror.ErrInvalidUserID) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
		}
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	items := make([]*socialv1message.FeedItem, 0, len(res.Items))
	for _, it := range res.Items {
		items = append(items, mapFeedItemDTOToProto(it))
	}

	return &socialv1message.SearchPostsResponse{
		Items:      items,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}, nil
}

func (h *GRPCHandler) ReactTarget(
	ctx context.Context,
	req *socialv1message.ReactTargetRequest,
) (*socialv1message.ReactTargetResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "authentication required")
	}

	reactionTypeStr := req.GetReactionType().String()
	// Strip "REACTION_TYPE_" prefix if present
	if len(reactionTypeStr) > 14 && reactionTypeStr[:14] == "REACTION_TYPE_" {
		reactionTypeStr = reactionTypeStr[14:]
	}

	res, err := h.reactTargetHandler.Handle(ctx, command.ReactTargetCommand{
		UserID:       actor.UserID,
		FeedItemID:   req.GetFeedItemId(),
		ReactionType: reactionTypeStr,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}

	return &socialv1message.ReactTargetResponse{
		Success:         true,
		CurrentReaction: res.CurrentReaction,
	}, nil
}

func (h *GRPCHandler) ListReactions(
	ctx context.Context,
	req *socialv1message.ListReactionsRequest,
) (*socialv1message.ListReactionsResponse, error) {
	if req.GetFeedItemId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, "feed_item_id required")
	}

	res, err := h.listReactionsHandler.Handle(ctx, query.ListReactionsQuery{
		FeedItemID: req.GetFeedItemId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	pbReactions := make([]*socialv1message.ReactionItem, 0, len(res.Reactions))
	for _, r := range res.Reactions {
		reactionTypeEnum := socialv1message.ReactionType_REACTION_TYPE_UNSPECIFIED
		switch r.ReactionType {
		case "LIKE":
			reactionTypeEnum = socialv1message.ReactionType_REACTION_TYPE_LIKE
		case "FIRE":
			reactionTypeEnum = socialv1message.ReactionType_REACTION_TYPE_FIRE
		case "MUSCLE":
			reactionTypeEnum = socialv1message.ReactionType_REACTION_TYPE_MUSCLE
		case "CLAP":
			reactionTypeEnum = socialv1message.ReactionType_REACTION_TYPE_CLAP
		}

		pbReactions = append(pbReactions, &socialv1message.ReactionItem{
			UserId:          r.UserID,
			AuthorName:      r.AuthorName,
			AuthorAvatarUrl: r.AuthorAvatarURL,
			ReactionType:    reactionTypeEnum,
			CreatedAt:       timestamppb.New(r.CreatedAt),
		})
	}

	return &socialv1message.ListReactionsResponse{
		Reactions:  pbReactions,
		TotalCount: res.TotalCount,
	}, nil
}

func (h *GRPCHandler) AddComment(
	ctx context.Context,
	req *socialv1message.AddCommentRequest,
) (*socialv1message.AddCommentResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "authentication required")
	}

	var parentID *string
	if req.ParentId != nil && *req.ParentId != "" {
		parentID = req.ParentId
	}

	res, err := h.addCommentHandler.Handle(ctx, command.AddCommentCommand{
		UserID:     actor.UserID,
		FeedItemID: req.GetFeedItemId(),
		Content:    req.GetContent(),
		ParentID:   parentID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}

	c := res.Comment
	return &socialv1message.AddCommentResponse{
		Comment: &socialv1message.CommentItem{
			Id:         c.ID(),
			UserId:     c.UserID(),
			FeedItemId: c.FeedItemID(),
			ParentId:   c.ParentID(),
			Content:    c.Content(),
			CreatedAt:  timestamppb.New(c.CreatedAt()),
		},
	}, nil
}

func (h *GRPCHandler) ListComments(
	ctx context.Context,
	req *socialv1message.ListCommentsRequest,
) (*socialv1message.ListCommentsResponse, error) {
	res, err := h.listCommentsHandler.Handle(ctx, query.ListCommentsQuery{
		FeedItemID: req.GetFeedItemId(),
		PageSize:   int(req.GetPageSize()),
		Cursor:     req.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	pbComments := make([]*socialv1message.CommentItem, 0, len(res.Comments))
	for _, c := range res.Comments {
		pbComments = append(pbComments, &socialv1message.CommentItem{
			Id:              c.ID,
			UserId:          c.UserID,
			AuthorName:      c.AuthorName,
			AuthorAvatarUrl: c.AuthorAvatarURL,
			FeedItemId:      c.FeedItemID,
			ParentId:        c.ParentID,
			Content:         c.Content,
			CreatedAt:       timestamppb.New(c.CreatedAt),
		})
	}

	return &socialv1message.ListCommentsResponse{
		Comments:   pbComments,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}, nil
}

func (h *GRPCHandler) DeleteComment(
	ctx context.Context,
	req *socialv1message.DeleteCommentRequest,
) (*socialv1message.DeleteCommentResponse, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "authentication required")
	}

	err = h.deleteCommentHandler.Handle(ctx, command.DeleteCommentCommand{
		CommentID: req.GetCommentId(),
		UserID:    actor.UserID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &socialv1message.DeleteCommentResponse{Success: true}, nil
}
