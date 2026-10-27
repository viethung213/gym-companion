package grpc

import (
	"context"

	"connectrpc.com/connect"
	solomatchv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/message"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/service/solomatchv1serviceconnect"
)

type ConnectSoloMatchHandler struct {
	grpcHandler *Handler
}

var _ solomatchv1serviceconnect.SoloMatchServiceHandler = (*ConnectSoloMatchHandler)(nil)

func NewConnectSoloMatchHandler(grpcHandler *Handler) *ConnectSoloMatchHandler {
	return &ConnectSoloMatchHandler{grpcHandler: grpcHandler}
}

func (c *ConnectSoloMatchHandler) JoinQueue(
	ctx context.Context,
	req *connect.Request[solomatchv1message.JoinQueueRequest],
	stream *connect.ServerStream[solomatchv1message.JoinQueueResponse],
) error {
	return c.grpcHandler.handleJoinQueue(ctx, req.Msg, stream.Send)
}

func (c *ConnectSoloMatchHandler) CancelQueue(
	ctx context.Context,
	req *connect.Request[solomatchv1message.CancelQueueRequest],
) (*connect.Response[solomatchv1message.CancelQueueResponse], error) {
	res, err := c.grpcHandler.CancelQueue(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectSoloMatchHandler) GetMatch(
	ctx context.Context,
	req *connect.Request[solomatchv1message.GetMatchRequest],
) (*connect.Response[solomatchv1message.GetMatchResponse], error) {
	res, err := c.grpcHandler.GetMatch(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectSoloMatchHandler) GetMatchHistory(
	ctx context.Context,
	req *connect.Request[solomatchv1message.GetMatchHistoryRequest],
) (*connect.Response[solomatchv1message.GetMatchHistoryResponse], error) {
	res, err := c.grpcHandler.GetMatchHistory(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
