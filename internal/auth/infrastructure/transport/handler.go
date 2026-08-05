package transport

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/command"
	"github.com/viethung213/gym-companion/internal/auth/application/query"
	authv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/message"
	authv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCHandler implements the gRPC service server using individual CQRS command and query handlers.
type GRPCHandler struct {
	authv1service.UnimplementedAuthServiceServer
	oauthLoginHandler       *command.OAuthLoginHandler
	logoutHandler           *command.LogoutHandler
	rotateKeysHandler       *command.RotateKeysHandler
	refreshTokenHandler     *command.RefreshTokenHandler
	getJWKSHandler          *query.GetJWKSHandler
	getOAuthLoginURLHandler *query.GetOAuthLoginURLHandler
}

// Compile-time interface verification
var _ authv1service.AuthServiceServer = (*GRPCHandler)(nil)

// NewGRPCHandler creates a new instance of GRPCHandler.
func NewGRPCHandler(
	oauthLoginHandler *command.OAuthLoginHandler,
	logoutHandler *command.LogoutHandler,
	rotateKeysHandler *command.RotateKeysHandler,
	refreshTokenHandler *command.RefreshTokenHandler,
	getJWKSHandler *query.GetJWKSHandler,
	getOAuthLoginURLHandler *query.GetOAuthLoginURLHandler,
) *GRPCHandler {
	return &GRPCHandler{
		oauthLoginHandler:       oauthLoginHandler,
		logoutHandler:           logoutHandler,
		rotateKeysHandler:       rotateKeysHandler,
		refreshTokenHandler:     refreshTokenHandler,
		getJWKSHandler:          getJWKSHandler,
		getOAuthLoginURLHandler: getOAuthLoginURLHandler,
	}
}

// RefreshToken authenticates a client via a valid refresh token and issues a new access token.
func (h *GRPCHandler) RefreshToken(
	ctx context.Context,
	req *authv1message.RefreshTokenRequest,
) (*authv1message.RefreshTokenResponse, error) {
	res, err := h.refreshTokenHandler.Handle(ctx, command.RefreshTokenCommand{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		if errors.Is(err, apperror.ErrUnauthorized) {
			return nil, status.Errorf(codes.Unauthenticated, "invalid or expired refresh token: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to refresh token: %v", err)
	}

	return &authv1message.RefreshTokenResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
	}, nil
}

// Logout revokes the session associated with the provided refresh token.
func (h *GRPCHandler) Logout(
	ctx context.Context,
	req *authv1message.LogoutRequest,
) (*authv1message.LogoutResponse, error) {
	userID, _ := ctx.Value(middleware.UserIDKey).(string)
	if userID == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-user-id"); len(vals) > 0 {
				userID = vals[0]
			}
		}
	}

	err := h.logoutHandler.Handle(ctx, command.LogoutCommand{
		RefreshToken: req.RefreshToken,
		UserID:       userID,
	})
	if err != nil {
		if errors.Is(err, apperror.ErrUnauthorized) {
			return &authv1message.LogoutResponse{
				Success: false,
			}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to logout: %v", err)
	}

	return &authv1message.LogoutResponse{
		Success: true,
	}, nil
}

// GetOAuthLoginURL generates the consent URL for the requested OAuth provider.
func (h *GRPCHandler) GetOAuthLoginURL(
	ctx context.Context,
	req *authv1message.GetOAuthLoginURLRequest,
) (*authv1message.GetOAuthLoginURLResponse, error) {
	url, err := h.getOAuthLoginURLHandler.Handle(ctx, query.GetOAuthLoginURLQuery{
		Provider:    req.Provider,
		RedirectURI: req.RedirectUri,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get login url: %v", err)
	}

	return &authv1message.GetOAuthLoginURLResponse{
		LoginUrl: url,
	}, nil
}

// RotateKeys generates a new active JWK pair and archives old ones.
func (h *GRPCHandler) RotateKeys(
	ctx context.Context,
	_ *authv1message.RotateKeysRequest,
) (*authv1message.RotateKeysResponse, error) {
	newKeyID, err := h.rotateKeysHandler.Handle(ctx, command.RotateKeysCommand{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to rotate keys: %v", err)
	}

	return &authv1message.RotateKeysResponse{
		Message: fmt.Sprintf("rotated keys successfully, new key ID: %s", newKeyID),
	}, nil
}

// GetJWKS retrieves all active public keys for JWT verification.
func (h *GRPCHandler) GetJWKS(
	ctx context.Context,
	_ *authv1message.GetJWKSRequest,
) (*authv1message.GetJWKSResponse, error) {
	keys, err := h.getJWKSHandler.Handle(ctx, query.GetJWKSQuery{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch JWKS: %v", err)
	}

	pbKeys := make([]*authv1message.JWKKey, 0, len(keys))
	for _, k := range keys {
		pbKeys = append(pbKeys, &authv1message.JWKKey{
			Kid: k.Kid,
			Kty: k.Kty,
			Alg: k.Alg,
			Use: k.Use,
			N:   k.N,
			E:   k.E,
		})
	}

	return &authv1message.GetJWKSResponse{
		Keys: pbKeys,
	}, nil
}

// LoginWithOAuth processes OAuth codes for standard authentication.
func (h *GRPCHandler) LoginWithOAuth(
	ctx context.Context,
	req *authv1message.LoginWithOAuthRequest,
) (*authv1message.LoginWithOAuthResponse, error) {
	accessToken, refreshToken, userID, err := h.oauthLoginHandler.Handle(
		ctx,
		command.OAuthLoginCommand{
			Provider:    req.Provider,
			Code:        req.Code,
			RedirectURI: req.RedirectUri,
			State:       req.State,
		},
	)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "oauth login failed: %v", err)
	}

	return &authv1message.LoginWithOAuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserId:       userID,
	}, nil
}

// --- ConnectRPC Adapter ---

type ConnectAuthHandler struct {
	grpcHandler *GRPCHandler
}

var _ authv1serviceconnect.AuthServiceHandler = (*ConnectAuthHandler)(nil)

func NewConnectAuthHandler(grpcHandler *GRPCHandler) authv1serviceconnect.AuthServiceHandler {
	return &ConnectAuthHandler{grpcHandler: grpcHandler}
}

func (c *ConnectAuthHandler) RefreshToken(
	ctx context.Context,
	req *connect.Request[authv1message.RefreshTokenRequest],
) (*connect.Response[authv1message.RefreshTokenResponse], error) {
	res, err := c.grpcHandler.RefreshToken(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) Logout(
	ctx context.Context,
	req *connect.Request[authv1message.LogoutRequest],
) (*connect.Response[authv1message.LogoutResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	res, err := c.grpcHandler.Logout(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) GetOAuthLoginURL(
	ctx context.Context,
	req *connect.Request[authv1message.GetOAuthLoginURLRequest],
) (*connect.Response[authv1message.GetOAuthLoginURLResponse], error) {
	res, err := c.grpcHandler.GetOAuthLoginURL(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) RotateKeys(
	ctx context.Context,
	req *connect.Request[authv1message.RotateKeysRequest],
) (*connect.Response[authv1message.RotateKeysResponse], error) {
	res, err := c.grpcHandler.RotateKeys(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) GetJWKS(
	ctx context.Context,
	req *connect.Request[authv1message.GetJWKSRequest],
) (*connect.Response[authv1message.GetJWKSResponse], error) {
	res, err := c.grpcHandler.GetJWKS(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) LoginWithOAuth(
	ctx context.Context,
	req *connect.Request[authv1message.LoginWithOAuthRequest],
) (*connect.Response[authv1message.LoginWithOAuthResponse], error) {
	res, err := c.grpcHandler.LoginWithOAuth(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
