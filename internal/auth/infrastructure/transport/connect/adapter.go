package connect

import (
	"context"

	"connectrpc.com/connect"
	authv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/message"
	authv1serviceconnect "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/connectutil"
	grpcAuth "github.com/viethung213/gym-companion/internal/auth/infrastructure/transport/grpc"
)

// Adapter wraps GRPCHandler to implement authv1serviceconnect.AuthServiceHandler.
type Adapter struct {
	authv1serviceconnect.UnimplementedAuthServiceHandler
	grpc *grpcAuth.GRPCHandler
}

var _ authv1serviceconnect.AuthServiceHandler = (*Adapter)(nil)

// NewAdapter creates a new Connect adapter wrapping the existing gRPC handler.
func NewAdapter(grpc *grpcAuth.GRPCHandler) *Adapter {
	return &Adapter{grpc: grpc}
}

func (a *Adapter) RefreshToken(
	ctx context.Context,
	req *connect.Request[authv1message.RefreshTokenRequest],
) (*connect.Response[authv1message.RefreshTokenResponse], error) {
	resp, err := a.grpc.RefreshToken(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) Logout(
	ctx context.Context,
	req *connect.Request[authv1message.LogoutRequest],
) (*connect.Response[authv1message.LogoutResponse], error) {
	resp, err := a.grpc.Logout(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetOAuthLoginURL(
	ctx context.Context,
	req *connect.Request[authv1message.GetOAuthLoginURLRequest],
) (*connect.Response[authv1message.GetOAuthLoginURLResponse], error) {
	resp, err := a.grpc.GetOAuthLoginURL(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) RotateKeys(
	ctx context.Context,
	req *connect.Request[authv1message.RotateKeysRequest],
) (*connect.Response[authv1message.RotateKeysResponse], error) {
	resp, err := a.grpc.RotateKeys(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetJWKS(
	ctx context.Context,
	req *connect.Request[authv1message.GetJWKSRequest],
) (*connect.Response[authv1message.GetJWKSResponse], error) {
	resp, err := a.grpc.GetJWKS(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) LoginWithOAuth(
	ctx context.Context,
	req *connect.Request[authv1message.LoginWithOAuthRequest],
) (*connect.Response[authv1message.LoginWithOAuthResponse], error) {
	resp, err := a.grpc.LoginWithOAuth(ctx, req.Msg)
	if err != nil {
		return nil, connectutil.FromGRPCError(err)
	}
	return connect.NewResponse(resp), nil
}
