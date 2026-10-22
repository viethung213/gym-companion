package grpc

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/gamification/application/query"
	"github.com/viethung213/gym-companion/internal/gamification/transport/consumer"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
	gamificationv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/service/gamificationv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler implements the Gamification gRPC/Connect service.
type Handler struct {
	getMyXpHandler              *query.GetMyXpHandler
	getXpHistoryHandler         *query.GetXpHistoryHandler
	getStreakHandler            *query.GetStreakHandler
	getStreakHistoryHandler     *query.GetStreakHistoryHandler
	getGlobalLeaderboardHandler *query.GetGlobalLeaderboardHandler
	getMyBadgesHandler          *query.GetMyBadgesHandler
}

// Compile-time interface check.
var _ gamificationv1serviceconnect.GamificationServiceHandler = (*Handler)(nil)

// NewHandler constructs a new Handler.
func NewHandler(
	getMyXpHandler *query.GetMyXpHandler,
	getXpHistoryHandler *query.GetXpHistoryHandler,
	getStreakHandler *query.GetStreakHandler,
	getStreakHistoryHandler *query.GetStreakHistoryHandler,
	getGlobalLeaderboardHandler *query.GetGlobalLeaderboardHandler,
	getMyBadgesHandler *query.GetMyBadgesHandler,
) *Handler {
	return &Handler{
		getMyXpHandler:              getMyXpHandler,
		getXpHistoryHandler:         getXpHistoryHandler,
		getStreakHandler:            getStreakHandler,
		getStreakHistoryHandler:     getStreakHistoryHandler,
		getGlobalLeaderboardHandler: getGlobalLeaderboardHandler,
		getMyBadgesHandler:          getMyBadgesHandler,
	}
}

// NewGRPCHandler constructs a new GRPCHandler for compatibility.
func NewGRPCHandler(
	getMyXpHandler *query.GetMyXpHandler,
	getXpHistoryHandler *query.GetXpHistoryHandler,
	getStreakHandler *query.GetStreakHandler,
	getStreakHistoryHandler *query.GetStreakHistoryHandler,
	getGlobalLeaderboardHandler *query.GetGlobalLeaderboardHandler,
	getMyBadgesHandler *query.GetMyBadgesHandler,
) *Handler {
	return NewHandler(
		getMyXpHandler,
		getXpHistoryHandler,
		getStreakHandler,
		getStreakHistoryHandler,
		getGlobalLeaderboardHandler,
		getMyBadgesHandler,
	)
}

// GetMyXp fetches current XP, level, and next-level progress.
func (h *Handler) GetMyXp(
	ctx context.Context,
	_ *connect.Request[gamificationv1message.GetMyXpRequest],
) (*connect.Response[gamificationv1message.GetMyXpResponse], error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	res, err := h.getMyXpHandler.Handle(ctx, query.GetMyXpQuery{UserID: userID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamificationv1message.GetMyXpResponse{
		XpSummary: &gamificationv1message.UserXpSummary{
			UserId:                  userID,
			Xp:                      res.Xp,
			Level:                   res.Level,
			XpToNextLevel:           res.XpToNextLevel,
			LevelProgressPercentage: float32(res.ProgressPercentage),
			UpdatedAt:               timestamppb.New(res.UpdatedAt),
		},
	}), nil
}

// GetXpHistory returns paginated XP history entries.
func (h *Handler) GetXpHistory(
	ctx context.Context,
	req *connect.Request[gamificationv1message.GetXpHistoryRequest],
) (*connect.Response[gamificationv1message.GetXpHistoryResponse], error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	pageSize := int(req.Msg.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	page := int(req.Msg.Page)
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	res, err := h.getXpHistoryHandler.Handle(ctx, query.GetXpHistoryQuery{
		UserID: userID,
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	entries := make([]*gamificationv1message.XpHistoryEntry, 0, len(res.Entries))
	for _, e := range res.Entries {
		entries = append(entries, &gamificationv1message.XpHistoryEntry{
			Id:           e.ID,
			UserId:       e.UserID,
			Amount:       e.DeltaXp,
			Source:       e.Source,
			SessionId:    e.SessionID,
			ActivityDate: e.ActivityDate,
			CreatedAt:    timestamppb.New(e.CreatedAt),
		})
	}

	return connect.NewResponse(&gamificationv1message.GetXpHistoryResponse{
		Entries:    entries,
		TotalCount: int32(res.Total),
		Page:       int32(page),
		PageSize:   int32(pageSize),
	}), nil
}

// GetStreak returns the user's active streak status.
func (h *Handler) GetStreak(
	ctx context.Context,
	_ *connect.Request[gamificationv1message.GetStreakRequest],
) (*connect.Response[gamificationv1message.GetStreakResponse], error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	currentLocalDate := consumer.ResolveLocalDate(time.Now())
	res, err := h.getStreakHandler.Handle(ctx, query.GetStreakQuery{
		UserID:           userID,
		CurrentLocalDate: currentLocalDate,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&gamificationv1message.GetStreakResponse{
		StreakSummary: &gamificationv1message.StreakSummary{
			UserId:         userID,
			CurrentStreak:  res.CurrentStreak,
			LongestStreak:  res.LongestStreak,
			LastActiveDate: res.LastActivityDate,
			IsActiveToday:  res.IsActiveToday,
			UpdatedAt:      timestamppb.New(res.UpdatedAt),
		},
	}), nil
}

// GetStreakHistory returns the recorded activity dates (heatmap).
func (h *Handler) GetStreakHistory(
	ctx context.Context,
	req *connect.Request[gamificationv1message.GetStreakHistoryRequest],
) (*connect.Response[gamificationv1message.GetStreakHistoryResponse], error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	limit := int(req.Msg.LimitDays)
	if limit <= 0 {
		limit = 30
	}

	res, err := h.getStreakHistoryHandler.Handle(ctx, query.GetStreakHistoryQuery{
		UserID: userID,
		Limit:  limit,
		Offset: 0,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	entries := make([]*gamificationv1message.StreakHistoryEntry, 0, len(res.Entries))
	for _, e := range res.Entries {
		entries = append(entries, &gamificationv1message.StreakHistoryEntry{
			Id:           e.ID,
			UserId:       e.UserID,
			ActivityDate: e.ActivityDate,
			ActivityType: e.ActivityType,
			CreatedAt:    timestamppb.New(e.CreatedAt),
		})
	}

	return connect.NewResponse(&gamificationv1message.GetStreakHistoryResponse{
		Entries: entries,
	}), nil
}

// GetGlobalLeaderboard returns Top 50 and caller's standing.
func (h *Handler) GetGlobalLeaderboard(
	ctx context.Context,
	req *connect.Request[gamificationv1message.GetGlobalLeaderboardRequest],
) (*connect.Response[gamificationv1message.GetGlobalLeaderboardResponse], error) {
	var callerID string
	if id, ok := ctx.Value(middleware.UserIDKey).(string); ok {
		callerID = id
	}

	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 50
	}

	res, err := h.getGlobalLeaderboardHandler.Handle(ctx, query.GetGlobalLeaderboardQuery{
		UserID: callerID,
		Limit:  limit,
		Offset: 0,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	topEntries := make([]*gamificationv1message.LeaderboardEntry, 0, len(res.TopUsers))
	for _, u := range res.TopUsers {
		topEntries = append(topEntries, &gamificationv1message.LeaderboardEntry{
			Rank:        int32(u.Rank),
			UserId:      u.UserID,
			DisplayName: u.DisplayName,
			AvatarUrl:   u.AvatarURL,
			Xp:          u.Xp,
			Level:       u.Level,
			UpdatedAt:   timestamppb.Now(),
		})
	}

	var myStanding *gamificationv1message.LeaderboardEntry
	if res.MyEntry != nil {
		myStanding = &gamificationv1message.LeaderboardEntry{
			Rank:        int32(res.MyRank),
			UserId:      res.MyEntry.UserID,
			DisplayName: res.MyEntry.DisplayName,
			AvatarUrl:   res.MyEntry.AvatarURL,
			Xp:          res.MyEntry.Xp,
			Level:       res.MyEntry.Level,
			UpdatedAt:   timestamppb.Now(),
		}
	}

	return connect.NewResponse(&gamificationv1message.GetGlobalLeaderboardResponse{
		TopEntries: topEntries,
		MyStanding: myStanding,
	}), nil
}

// GetMyBadges returns all badges from the system catalog along with caller unlock status.
func (h *Handler) GetMyBadges(
	ctx context.Context,
	_ *connect.Request[gamificationv1message.GetMyBadgesRequest],
) (*connect.Response[gamificationv1message.GetMyBadgesResponse], error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	res, err := h.getMyBadgesHandler.Handle(ctx, query.GetMyBadgesQuery{UserID: userID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	unlockedMap := make(map[string]time.Time, len(res.Badges))
	for _, b := range res.Badges {
		unlockedMap[b.BadgeCode()] = b.UnlockedAt()
	}

	catalog := vo.MasterBadgeCatalog()
	badges := make([]*gamificationv1message.BadgeItem, 0, len(catalog))
	for _, def := range catalog {
		unlockedAtTime, isUnlocked := unlockedMap[def.Code]
		var pbUnlockedAt *timestamppb.Timestamp
		if isUnlocked {
			pbUnlockedAt = timestamppb.New(unlockedAtTime)
		}

		badges = append(badges, &gamificationv1message.BadgeItem{
			BadgeCode:   def.Code,
			Name:        def.Name,
			Description: def.Description,
			Category:    def.Category,
			IconUrl:     def.IconURL,
			IsUnlocked:  isUnlocked,
			UnlockedAt:  pbUnlockedAt,
		})
	}

	return connect.NewResponse(&gamificationv1message.GetMyBadgesResponse{
		Badges:         badges,
		TotalUnlocked:  int32(len(res.Badges)),
		TotalAvailable: int32(len(catalog)),
	}), nil
}

func getUserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, middleware.ErrUnauthorized)
	}
	return userID, nil
}
