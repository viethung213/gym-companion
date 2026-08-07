package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	notificationv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/message"
	notificationv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/service/notificationv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/query"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ notificationv1service.NotificationServiceServer = (*GRPCHandler)(nil)

type GRPCHandler struct {
	notificationv1service.UnimplementedNotificationServiceServer
	sendPushHandler       *command.SendPushNotificationHandler
	registerDeviceHandler *command.RegisterDeviceTokenHandler
	updateSettingsHandler *command.UpdateNotificationSettingsHandler
	markAsReadHandler     *command.MarkNotificationAsReadHandler
	getSettingsHandler    *query.GetNotificationSettingsHandler
	listNotifsHandler     *query.ListNotificationsHandler
}

func NewGRPCHandler(
	sendPushHandler *command.SendPushNotificationHandler,
	registerDeviceHandler *command.RegisterDeviceTokenHandler,
	updateSettingsHandler *command.UpdateNotificationSettingsHandler,
	markAsReadHandler *command.MarkNotificationAsReadHandler,
	getSettingsHandler *query.GetNotificationSettingsHandler,
	listNotifsHandler *query.ListNotificationsHandler,
) *GRPCHandler {
	return &GRPCHandler{
		sendPushHandler:       sendPushHandler,
		registerDeviceHandler: registerDeviceHandler,
		updateSettingsHandler: updateSettingsHandler,
		markAsReadHandler:     markAsReadHandler,
		getSettingsHandler:    getSettingsHandler,
		listNotifsHandler:     listNotifsHandler,
	}
}

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, derror.ErrEmptyUserID),
		errors.Is(err, derror.ErrEmptyDeviceToken),
		errors.Is(err, derror.ErrEmptyTitle),
		errors.Is(err, derror.ErrInvalidTimeFormat):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, derror.ErrDeviceNotFound),
		errors.Is(err, derror.ErrSettingNotFound),
		errors.Is(err, derror.ErrNotificationNotFound):
		return status.Error(codes.NotFound, err.Error())

	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (h *GRPCHandler) SendPushNotification(
	ctx context.Context,
	req *notificationv1message.SendPushNotificationRequest,
) (*notificationv1message.SendPushNotificationResponse, error) {
	resp, err := h.sendPushHandler.Handle(ctx, command.SendPushNotificationCommand{
		UserID: req.GetUserId(),
		Title:  req.GetTitle(),
		Body:   req.GetBody(),
		Data:   req.GetData(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &notificationv1message.SendPushNotificationResponse{
		NotificationId: resp.NotificationID,
		Status:         resp.Status,
		Message:        resp.Message,
	}, nil
}

func (h *GRPCHandler) RegisterDeviceToken(
	ctx context.Context,
	req *notificationv1message.RegisterDeviceTokenRequest,
) (*notificationv1message.RegisterDeviceTokenResponse, error) {
	err := h.registerDeviceHandler.Handle(ctx, command.RegisterDeviceTokenCommand{
		UserID:      req.GetUserId(),
		DeviceToken: req.GetDeviceToken(),
		DeviceType:  req.GetDeviceType(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &notificationv1message.RegisterDeviceTokenResponse{
		Success: true,
		Message: "Device token registered successfully",
	}, nil
}

func (h *GRPCHandler) GetNotificationSettings(
	ctx context.Context,
	req *notificationv1message.GetNotificationSettingsRequest,
) (*notificationv1message.GetNotificationSettingsResponse, error) {
	setting, err := h.getSettingsHandler.Handle(ctx, query.GetNotificationSettingsQuery{
		UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &notificationv1message.GetNotificationSettingsResponse{
		UserId:          setting.UserID(),
		EnablePush:      setting.EnablePush(),
		EnableEmail:     setting.EnableEmail(),
		EnableSms:       setting.EnableSMS(),
		QuietHoursStart: setting.QuietHoursStart(),
		QuietHoursEnd:   setting.QuietHoursEnd(),
	}, nil
}

func (h *GRPCHandler) UpdateNotificationSettings(
	ctx context.Context,
	req *notificationv1message.UpdateNotificationSettingsRequest,
) (*notificationv1message.UpdateNotificationSettingsResponse, error) {
	err := h.updateSettingsHandler.Handle(ctx, command.UpdateNotificationSettingsCommand{
		UserID:          req.GetUserId(),
		EnablePush:      req.GetEnablePush(),
		EnableEmail:     req.GetEnableEmail(),
		EnableSMS:       req.GetEnableSms(),
		QuietHoursStart: req.GetQuietHoursStart(),
		QuietHoursEnd:   req.GetQuietHoursEnd(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &notificationv1message.UpdateNotificationSettingsResponse{
		Success: true,
		Message: "Notification settings updated successfully",
	}, nil
}

func (h *GRPCHandler) ListNotifications(
	ctx context.Context,
	req *notificationv1message.ListNotificationsRequest,
) (*notificationv1message.ListNotificationsResponse, error) {
	result, err := h.listNotifsHandler.Handle(ctx, query.ListNotificationsQuery{
		UserID: req.GetUserId(),
		Limit:  req.GetLimit(),
		Offset: req.GetOffset(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	pbItems := make([]*notificationv1message.NotificationItem, 0, len(result.Items))
	for _, item := range result.Items {
		pbItems = append(pbItems, &notificationv1message.NotificationItem{
			NotificationId: item.ID(),
			Title:          item.Title(),
			Body:           item.Body(),
			Data:           item.Data(),
			IsRead:         item.IsRead(),
			CreatedAt:      item.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return &notificationv1message.ListNotificationsResponse{
		Notifications: pbItems,
		TotalCount:    result.TotalCount,
	}, nil
}

func (h *GRPCHandler) MarkNotificationAsRead(
	ctx context.Context,
	req *notificationv1message.MarkNotificationAsReadRequest,
) (*notificationv1message.MarkNotificationAsReadResponse, error) {
	err := h.markAsReadHandler.Handle(ctx, command.MarkNotificationAsReadCommand{
		UserID:         req.GetUserId(),
		NotificationID: req.GetNotificationId(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &notificationv1message.MarkNotificationAsReadResponse{
		Success: true,
		Message: "Notification marked as read successfully",
	}, nil
}

// --- ConnectRPC Adapter ---

type ConnectNotificationHandler struct {
	grpcHandler *GRPCHandler
}

var _ notificationv1serviceconnect.NotificationServiceHandler = (*ConnectNotificationHandler)(nil)

func NewConnectNotificationHandler(grpcHandler *GRPCHandler) notificationv1serviceconnect.NotificationServiceHandler {
	return &ConnectNotificationHandler{grpcHandler: grpcHandler}
}

func (c *ConnectNotificationHandler) SendPushNotification(
	ctx context.Context,
	req *connect.Request[notificationv1message.SendPushNotificationRequest],
) (*connect.Response[notificationv1message.SendPushNotificationResponse], error) {
	res, err := c.grpcHandler.SendPushNotification(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNotificationHandler) RegisterDeviceToken(
	ctx context.Context,
	req *connect.Request[notificationv1message.RegisterDeviceTokenRequest],
) (*connect.Response[notificationv1message.RegisterDeviceTokenResponse], error) {
	res, err := c.grpcHandler.RegisterDeviceToken(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNotificationHandler) GetNotificationSettings(
	ctx context.Context,
	req *connect.Request[notificationv1message.GetNotificationSettingsRequest],
) (*connect.Response[notificationv1message.GetNotificationSettingsResponse], error) {
	res, err := c.grpcHandler.GetNotificationSettings(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNotificationHandler) UpdateNotificationSettings(
	ctx context.Context,
	req *connect.Request[notificationv1message.UpdateNotificationSettingsRequest],
) (*connect.Response[notificationv1message.UpdateNotificationSettingsResponse], error) {
	res, err := c.grpcHandler.UpdateNotificationSettings(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNotificationHandler) ListNotifications(
	ctx context.Context,
	req *connect.Request[notificationv1message.ListNotificationsRequest],
) (*connect.Response[notificationv1message.ListNotificationsResponse], error) {
	res, err := c.grpcHandler.ListNotifications(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectNotificationHandler) MarkNotificationAsRead(
	ctx context.Context,
	req *connect.Request[notificationv1message.MarkNotificationAsReadRequest],
) (*connect.Response[notificationv1message.MarkNotificationAsReadResponse], error) {
	res, err := c.grpcHandler.MarkNotificationAsRead(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
