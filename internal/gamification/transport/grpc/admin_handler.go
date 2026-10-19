package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/application/query"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	gamificationv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/service/gamificationv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AdminHandler implements gamificationv1serviceconnect.GamificationAdminServiceHandler.
type AdminHandler struct {
	getAdminUserGamificationHandler *query.GetAdminUserGamificationHandler
	adminAdjustCoinsHandler         *command.AdminAdjustCoinsHandler
	adminRepairStreakHandler        *command.AdminRepairStreakHandler
}

// Compile-time interface check.
var _ gamificationv1serviceconnect.GamificationAdminServiceHandler = (*AdminHandler)(nil)

// NewAdminHandler constructs a new AdminHandler.
func NewAdminHandler(
	getAdminUserGamificationHandler *query.GetAdminUserGamificationHandler,
	adminAdjustCoinsHandler *command.AdminAdjustCoinsHandler,
	adminRepairStreakHandler *command.AdminRepairStreakHandler,
) *AdminHandler {
	return &AdminHandler{
		getAdminUserGamificationHandler: getAdminUserGamificationHandler,
		adminAdjustCoinsHandler:         adminAdjustCoinsHandler,
		adminRepairStreakHandler:        adminRepairStreakHandler,
	}
}

// GetAdminUserGamification fetches summary gamification info for admin user inspection.
func (h *AdminHandler) GetAdminUserGamification(
	ctx context.Context,
	req *connect.Request[gamificationv1message.GetAdminUserGamificationRequest],
) (*connect.Response[gamificationv1message.GetAdminUserGamificationResponse], error) {
	if _, err := checkAdminPermission(ctx); err != nil {
		return nil, err
	}

	res, err := h.getAdminUserGamificationHandler.Handle(ctx, query.GetAdminUserGamificationQuery{
		UserID: req.Msg.GetUserId(),
	})
	if err != nil {
		return nil, mapAdminError(err)
	}

	return connect.NewResponse(&gamificationv1message.GetAdminUserGamificationResponse{
		Summary: &gamificationv1message.AdminUserGamificationSummary{
			UserId:         res.UserID,
			Xp:             res.Xp,
			Level:          res.Level,
			CurrentStreak:  res.CurrentStreak,
			LongestStreak:  res.LongestStreak,
			CoinBalance:    res.CoinBalance,
			LastActiveDate: res.LastActiveDate,
			UpdatedAt:      timestamppb.New(res.UpdatedAt),
		},
	}), nil
}

// AdminAdjustCoins directly adjusts user FitCoins.
func (h *AdminHandler) AdminAdjustCoins(
	ctx context.Context,
	req *connect.Request[gamificationv1message.AdminAdjustCoinsRequest],
) (*connect.Response[gamificationv1message.AdminAdjustCoinsResponse], error) {
	actor, err := checkAdminPermission(ctx)
	if err != nil {
		return nil, err
	}

	res, err := h.adminAdjustCoinsHandler.Handle(ctx, command.AdminAdjustCoinsCommand{
		UserID:  req.Msg.GetUserId(),
		AdminID: actor.UserID,
		Amount:  req.Msg.GetAmount(),
		Reason:  req.Msg.GetReason(),
	})
	if err != nil {
		return nil, mapAdminError(err)
	}

	return connect.NewResponse(&gamificationv1message.AdminAdjustCoinsResponse{
		UserId:       res.UserID,
		BalanceAfter: res.BalanceAfter,
	}), nil
}

// AdminRepairStreak directly repairs user workout streak.
func (h *AdminHandler) AdminRepairStreak(
	ctx context.Context,
	req *connect.Request[gamificationv1message.AdminRepairStreakRequest],
) (*connect.Response[gamificationv1message.AdminRepairStreakResponse], error) {
	actor, err := checkAdminPermission(ctx)
	if err != nil {
		return nil, err
	}

	res, err := h.adminRepairStreakHandler.Handle(ctx, command.AdminRepairStreakCommand{
		UserID:        req.Msg.GetUserId(),
		AdminID:       actor.UserID,
		TargetStreak:  req.Msg.GetTargetStreak(),
		Reason:        req.Msg.GetReason(),
		EffectiveDate: req.Msg.GetEffectiveDate(),
	})
	if err != nil {
		return nil, mapAdminError(err)
	}

	return connect.NewResponse(&gamificationv1message.AdminRepairStreakResponse{
		UserId:        res.UserID,
		CurrentStreak: res.CurrentStreak,
		LongestStreak: res.LongestStreak,
	}), nil
}

func checkAdminPermission(ctx context.Context) (*middleware.Actor, error) {
	actor, err := middleware.RequireAdmin(ctx)
	if err != nil {
		if errors.Is(err, middleware.ErrUnauthorized) {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	if !actor.IsAdmin() {
		return nil, connect.NewError(connect.CodePermissionDenied, middleware.ErrForbidden)
	}
	return &actor, nil
}

func mapAdminError(err error) *connect.Error {
	if errors.Is(err, aggregate.ErrEmptyUserID) ||
		errors.Is(err, aggregate.ErrInvalidUserID) ||
		errors.Is(err, aggregate.ErrZeroCoinAmount) ||
		errors.Is(err, aggregate.ErrCoinAmountOutOfRange) ||
		errors.Is(err, aggregate.ErrInsufficientBalance) ||
		errors.Is(err, aggregate.ErrNegativeStreak) ||
		errors.Is(err, aggregate.ErrStreakExceedsLimit) ||
		errors.Is(err, aggregate.ErrInvalidDateFormat) ||
		errors.Is(err, aggregate.ErrFutureActivityDate) {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
