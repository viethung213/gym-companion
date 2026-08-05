// Package connect provides the Connect-protocol transport handler for AuthService.
// Each method delegates to the existing gRPC handler, converting req/resp wrappers only.
package connect

import (
	"context"

	"connectrpc.com/connect"

	authv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/message"
	authv1serviceconnect "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	grpctransport "github.com/viethung213/gym-companion/internal/auth/infrastructure/transport/grpc"
)

// Handler delegates Connect-protocol calls to the existing gRPC handler.
type Handler struct {
	authv1serviceconnect.UnimplementedAuthServiceHandler
	grpc *grpctransport.GRPCHandler
}

var _ authv1serviceconnect.AuthServiceHandler = (*Handler)(nil)

func NewHandler(grpc *grpctransport.GRPCHandler) *Handler {
	return &Handler{grpc: grpc}
}

func (h *Handler) LoginWithOAuth(ctx context.Context, req *connect.Request[authv1message.LoginWithOAuthRequest]) (*connect.Response[authv1message.LoginWithOAuthResponse], error) {
	resp, err := h.grpc.LoginWithOAuth(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) RefreshToken(ctx context.Context, req *connect.Request[authv1message.RefreshTokenRequest]) (*connect.Response[authv1message.RefreshTokenResponse], error) {
	resp, err := h.grpc.RefreshToken(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) Logout(ctx context.Context, req *connect.Request[authv1message.LogoutRequest]) (*connect.Response[authv1message.LogoutResponse], error) {
	resp, err := h.grpc.Logout(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetOAuthLoginURL(ctx context.Context, req *connect.Request[authv1message.GetOAuthLoginURLRequest]) (*connect.Response[authv1message.GetOAuthLoginURLResponse], error) {
	resp, err := h.grpc.GetOAuthLoginURL(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) GetJWKS(ctx context.Context, req *connect.Request[authv1message.GetJWKSRequest]) (*connect.Response[authv1message.GetJWKSResponse], error) {
	resp, err := h.grpc.GetJWKS(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}

func (h *Handler) RotateKeys(ctx context.Context, req *connect.Request[authv1message.RotateKeysRequest]) (*connect.Response[authv1message.RotateKeysResponse], error) {
	resp, err := h.grpc.RotateKeys(ctx, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeOf(err), err)
	}
	return connect.NewResponse(resp), nil
}
