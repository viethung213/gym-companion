package grpc

import (
	"context"

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
	req *solomatchv1message.JoinQueueRequest,
	stream solomatchv1serviceconnect.SoloMatchServiceJoinQueueServerStream,
) error {
	return c.grpcHandler.handleJoinQueue(ctx, req, stream.Send)
}

func (c *ConnectSoloMatchHandler) CancelQueue(
	ctx context.Context,
	req *solomatchv1message.CancelQueueRequest,
) (*solomatchv1message.CancelQueueResponse, error) {
	res, err := c.grpcHandler.CancelQueue(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectSoloMatchHandler) GetActiveMatch(
	ctx context.Context,
	req *solomatchv1message.GetActiveMatchRequest,
) (*solomatchv1message.GetActiveMatchResponse, error) {
	res, err := c.grpcHandler.GetActiveMatch(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectSoloMatchHandler) GetMatch(
	ctx context.Context,
	req *solomatchv1message.GetMatchRequest,
) (*solomatchv1message.GetMatchResponse, error) {
	res, err := c.grpcHandler.GetMatch(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *ConnectSoloMatchHandler) GetMatchHistory(
	ctx context.Context,
	req *solomatchv1message.GetMatchHistoryRequest,
) (*solomatchv1message.GetMatchHistoryResponse, error) {
	res, err := c.grpcHandler.GetMatchHistory(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
