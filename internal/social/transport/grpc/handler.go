package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	socialv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/social/v1/service/socialv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/application/query"
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
	}
}

func (h *GRPCHandler) FollowUser(
	ctx context.Context,
	req *connect.Request[socialv1message.FollowUserRequest],
) (*connect.Response[socialv1message.FollowUserResponse], error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	err = h.followUserHandler.Handle(ctx, command.FollowUserCommand{
		FollowerID:  actor.UserID,
		FollowingID: req.Msg.GetFollowingId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&socialv1message.FollowUserResponse{
		Success: true,
		Message: "Followed successfully",
	}), nil
}

func (h *GRPCHandler) UnfollowUser(
	ctx context.Context,
	req *connect.Request[socialv1message.UnfollowUserRequest],
) (*connect.Response[socialv1message.UnfollowUserResponse], error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	err = h.unfollowUserHandler.Handle(ctx, command.UnfollowUserCommand{
		FollowerID:  actor.UserID,
		FollowingID: req.Msg.GetFollowingId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&socialv1message.UnfollowUserResponse{
		Success: true,
		Message: "Unfollowed successfully",
	}), nil
}

func (h *GRPCHandler) GetFollowers(
	ctx context.Context,
	req *connect.Request[socialv1message.GetFollowersRequest],
) (*connect.Response[socialv1message.GetFollowersResponse], error) {
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}

	targetUserID := req.Msg.GetUserId()
	if targetUserID == "" {
		targetUserID = currentUserID
	}
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id required or authentication required"))
	}

	res, err := h.getFollowersHandler.Handle(ctx, query.GetFollowersQuery{
		UserID:        targetUserID,
		CurrentUserID: currentUserID,
		PageSize:      int(req.Msg.GetPageSize()),
		Cursor:        req.Msg.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pbFollowers := make([]*socialv1message.UserSocialSummary, 0, len(res.Followers))
	for _, f := range res.Followers {
		pbFollowers = append(pbFollowers, &socialv1message.UserSocialSummary{
			UserId:      f.UserID,
			FullName:    f.FullName,
			AvatarUrl:   f.AvatarURL,
			IsFollowing: f.IsFollowing,
		})
	}

	return connect.NewResponse(&socialv1message.GetFollowersResponse{
		Followers:  pbFollowers,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}), nil
}

func (h *GRPCHandler) GetFollowing(
	ctx context.Context,
	req *connect.Request[socialv1message.GetFollowingRequest],
) (*connect.Response[socialv1message.GetFollowingResponse], error) {
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}

	targetUserID := req.Msg.GetUserId()
	if targetUserID == "" {
		targetUserID = currentUserID
	}
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id required or authentication required"))
	}

	res, err := h.getFollowingHandler.Handle(ctx, query.GetFollowingQuery{
		UserID:        targetUserID,
		CurrentUserID: currentUserID,
		PageSize:      int(req.Msg.GetPageSize()),
		Cursor:        req.Msg.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pbFollowing := make([]*socialv1message.UserSocialSummary, 0, len(res.Following))
	for _, f := range res.Following {
		pbFollowing = append(pbFollowing, &socialv1message.UserSocialSummary{
			UserId:      f.UserID,
			FullName:    f.FullName,
			AvatarUrl:   f.AvatarURL,
			IsFollowing: f.IsFollowing,
		})
	}

	return connect.NewResponse(&socialv1message.GetFollowingResponse{
		Following:  pbFollowing,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}), nil
}

func (h *GRPCHandler) GetSocialSummary(
	ctx context.Context,
	req *connect.Request[socialv1message.GetSocialSummaryRequest],
) (*connect.Response[socialv1message.GetSocialSummaryResponse], error) {
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}

	targetUserID := req.Msg.GetUserId()
	if targetUserID == "" {
		targetUserID = currentUserID
	}
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id required"))
	}

	res, err := h.getSocialSummaryHandler.Handle(ctx, query.GetSocialSummaryQuery{
		TargetUserID:  targetUserID,
		CurrentUserID: currentUserID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&socialv1message.GetSocialSummaryResponse{
		UserId:         res.UserID,
		FollowerCount:  res.FollowerCount,
		FollowingCount: res.FollowingCount,
		PostCount:      res.PostCount,
		IsFollowing:    res.IsFollowing,
	}), nil
}

func (h *GRPCHandler) SearchUsers(
	ctx context.Context,
	req *connect.Request[socialv1message.SearchUsersRequest],
) (*connect.Response[socialv1message.SearchUsersResponse], error) {
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}

	res, err := h.searchUsersHandler.Handle(ctx, query.SearchUsersQuery{
		CurrentUserID: currentUserID,
		Query:         req.Msg.GetQuery(),
		Role:          req.Msg.GetRole(),
		PageSize:      int(req.Msg.GetPageSize()),
		Cursor:        req.Msg.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	return connect.NewResponse(&socialv1message.SearchUsersResponse{
		Users:      pbUsers,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}), nil
}

func (h *GRPCHandler) CreatePost(
	ctx context.Context,
	req *connect.Request[socialv1message.CreatePostRequest],
) (*connect.Response[socialv1message.CreatePostResponse], error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	res, err := h.createPostHandler.Handle(ctx, command.CreatePostCommand{
		UserID:     actor.UserID,
		Caption:    req.Msg.GetCaption(),
		MediaURLs:  req.Msg.GetMediaUrls(),
		Visibility: req.Msg.GetVisibility(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	item := res.Item
	return connect.NewResponse(&socialv1message.CreatePostResponse{
		Item: &socialv1message.FeedItem{
			Id:            item.ID(),
			UserId:        item.UserID(),
			ItemType:      socialv1message.FeedItemType_FEED_ITEM_TYPE_POST,
			Caption:       item.Caption(),
			MediaUrls:     item.MediaURLs(),
			Visibility:    item.Visibility().String(),
			ReactionCount: item.ReactionCount(),
			CommentCount:  item.CommentCount(),
			CreatedAt:     timestamppb.New(item.CreatedAt()),
			UpdatedAt:     timestamppb.New(item.UpdatedAt()),
		},
	}), nil
}

func (h *GRPCHandler) DeleteFeedItem(
	ctx context.Context,
	req *connect.Request[socialv1message.DeleteFeedItemRequest],
) (*connect.Response[socialv1message.DeleteFeedItemResponse], error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	err = h.deleteFeedItemHandler.Handle(ctx, command.DeleteFeedItemCommand{
		FeedItemID: req.Msg.GetFeedItemId(),
		UserID:     actor.UserID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&socialv1message.DeleteFeedItemResponse{Success: true}), nil
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
	req *connect.Request[socialv1message.GetActivityFeedRequest],
) (*connect.Response[socialv1message.GetActivityFeedResponse], error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	res, err := h.getActivityFeedHandler.Handle(ctx, query.GetActivityFeedQuery{
		CurrentUserID: actor.UserID,
		PageSize:      int(req.Msg.GetPageSize()),
		Cursor:        req.Msg.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pbItems := make([]*socialv1message.FeedItem, 0, len(res.Items))
	for _, item := range res.Items {
		pbItems = append(pbItems, mapFeedItemDTOToProto(item))
	}

	return connect.NewResponse(&socialv1message.GetActivityFeedResponse{
		Items:      pbItems,
		NextCursor: res.NextCursor,
	}), nil
}

func (h *GRPCHandler) GetUserProfileFeed(
	ctx context.Context,
	req *connect.Request[socialv1message.GetUserProfileFeedRequest],
) (*connect.Response[socialv1message.GetUserProfileFeedResponse], error) {
	currentUserID := ""
	if actor, err := middleware.RequireAuthenticated(ctx); err == nil {
		currentUserID = actor.UserID
	}

	targetUserID := req.Msg.GetUserId()
	if targetUserID == "" {
		targetUserID = currentUserID
	}
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id required or authentication required"))
	}

	res, err := h.getUserProfileFeedHandler.Handle(ctx, query.GetUserProfileFeedQuery{
		TargetUserID:  targetUserID,
		CurrentUserID: currentUserID,
		PageSize:      int(req.Msg.GetPageSize()),
		Cursor:        req.Msg.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pbItems := make([]*socialv1message.FeedItem, 0, len(res.Items))
	for _, item := range res.Items {
		pbItems = append(pbItems, mapFeedItemDTOToProto(item))
	}

	return connect.NewResponse(&socialv1message.GetUserProfileFeedResponse{
		Items:      pbItems,
		NextCursor: res.NextCursor,
	}), nil
}

func (h *GRPCHandler) ReactTarget(
	ctx context.Context,
	req *connect.Request[socialv1message.ReactTargetRequest],
) (*connect.Response[socialv1message.ReactTargetResponse], error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	reactionTypeStr := req.Msg.GetReactionType().String()
	// Strip "REACTION_TYPE_" prefix if present
	if len(reactionTypeStr) > 14 && reactionTypeStr[:14] == "REACTION_TYPE_" {
		reactionTypeStr = reactionTypeStr[14:]
	}

	res, err := h.reactTargetHandler.Handle(ctx, command.ReactTargetCommand{
		UserID:       actor.UserID,
		FeedItemID:   req.Msg.GetFeedItemId(),
		ReactionType: reactionTypeStr,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&socialv1message.ReactTargetResponse{
		Success:         true,
		CurrentReaction: res.CurrentReaction,
	}), nil
}

func (h *GRPCHandler) ListReactions(
	ctx context.Context,
	req *connect.Request[socialv1message.ListReactionsRequest],
) (*connect.Response[socialv1message.ListReactionsResponse], error) {
	if req.Msg.GetFeedItemId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("feed_item_id required"))
	}

	res, err := h.listReactionsHandler.Handle(ctx, query.ListReactionsQuery{
		FeedItemID: req.Msg.GetFeedItemId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	return connect.NewResponse(&socialv1message.ListReactionsResponse{
		Reactions:  pbReactions,
		TotalCount: res.TotalCount,
	}), nil
}

func (h *GRPCHandler) AddComment(
	ctx context.Context,
	req *connect.Request[socialv1message.AddCommentRequest],
) (*connect.Response[socialv1message.AddCommentResponse], error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	var parentID *string
	if req.Msg.ParentId != nil && *req.Msg.ParentId != "" {
		parentID = req.Msg.ParentId
	}

	res, err := h.addCommentHandler.Handle(ctx, command.AddCommentCommand{
		UserID:     actor.UserID,
		FeedItemID: req.Msg.GetFeedItemId(),
		Content:    req.Msg.GetContent(),
		ParentID:   parentID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	c := res.Comment
	return connect.NewResponse(&socialv1message.AddCommentResponse{
		Comment: &socialv1message.CommentItem{
			Id:         c.ID(),
			UserId:     c.UserID(),
			FeedItemId: c.FeedItemID(),
			ParentId:   c.ParentID(),
			Content:    c.Content(),
			CreatedAt:  timestamppb.New(c.CreatedAt()),
		},
	}), nil
}

func (h *GRPCHandler) ListComments(
	ctx context.Context,
	req *connect.Request[socialv1message.ListCommentsRequest],
) (*connect.Response[socialv1message.ListCommentsResponse], error) {
	res, err := h.listCommentsHandler.Handle(ctx, query.ListCommentsQuery{
		FeedItemID: req.Msg.GetFeedItemId(),
		PageSize:   int(req.Msg.GetPageSize()),
		Cursor:     req.Msg.GetCursor(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	return connect.NewResponse(&socialv1message.ListCommentsResponse{
		Comments:   pbComments,
		NextCursor: res.NextCursor,
		TotalCount: res.TotalCount,
	}), nil
}

func (h *GRPCHandler) DeleteComment(
	ctx context.Context,
	req *connect.Request[socialv1message.DeleteCommentRequest],
) (*connect.Response[socialv1message.DeleteCommentResponse], error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	err = h.deleteCommentHandler.Handle(ctx, command.DeleteCommentCommand{
		CommentID: req.Msg.GetCommentId(),
		UserID:    actor.UserID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&socialv1message.DeleteCommentResponse{Success: true}), nil
}
