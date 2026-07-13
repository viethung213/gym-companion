package grpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/command"
	"github.com/viethung213/gym-companion/internal/auth/application/query"
	authv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/message"
	authv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service"
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
			return nil, status.Errorf(codes.Unauthenticated, "invalid or expired refresh token")
		}
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	return &authv1message.RefreshTokenResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
	}, nil
}

type contextKey string

const userIDContextKey contextKey = "userId"

func extractUserID(ctx context.Context) (string, error) {
	// 1. Try standard context key (camelCase) set by middleware later
	if val, ok := ctx.Value(userIDContextKey).(string); ok && val != "" {
		return val, nil
	}
	if val, ok := ctx.Value("userId").(string); ok && val != "" {
		return val, nil
	}

	// 2. Fallback to gRPC metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("missing request metadata")
	}

	metadataKeys := []string{"x-user-id", "user-id", "grpcgateway-x-user-id", "grpcgateway-user-id"}
	for _, key := range metadataKeys {
		if val := md.Get(key); len(val) > 0 && val[0] != "" {
			return val[0], nil
		}
	}

	return "", errors.New("missing user identity in context or metadata")
}

// Logout revokes a user's session token.
func (h *GRPCHandler) Logout(
	ctx context.Context,
	req *authv1message.LogoutRequest,
) (*authv1message.LogoutResponse, error) {
	userID, err := extractUserID(ctx)
	if err != nil {
		return &authv1message.LogoutResponse{
			Success: false,
			Message: fmt.Sprintf("failed to logout: %v", err),
		}, nil
	}

	err = h.logoutHandler.Handle(ctx, command.LogoutCommand{
		RefreshToken: req.RefreshToken,
		UserID:       userID,
	})
	if err != nil {
		return &authv1message.LogoutResponse{
			Success: false,
			Message: fmt.Sprintf("failed to logout: %v", err),
		}, nil
	}

	return &authv1message.LogoutResponse{
		Success: true,
		Message: "Logged out successfully",
	}, nil
}

// GetOAuthLoginURL generates redirect URL to sign in via Facebook or Google OAuth.
func (h *GRPCHandler) GetOAuthLoginURL(
	ctx context.Context,
	req *authv1message.GetOAuthLoginURLRequest,
) (*authv1message.GetOAuthLoginURLResponse, error) {
	urlStr, err := h.getOAuthLoginURLHandler.Handle(ctx, query.GetOAuthLoginURLQuery{
		Provider:    req.Provider,
		RedirectURI: req.RedirectUri,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to get login URL: %v", err)
	}

	return &authv1message.GetOAuthLoginURLResponse{
		LoginUrl: urlStr,
	}, nil
}

// RotateKeys generates a new key pair, publishes active state,
// and deprecates outdated signing keys.
func (h *GRPCHandler) RotateKeys(
	ctx context.Context,
	_ *authv1message.RotateKeysRequest,
) (*authv1message.RotateKeysResponse, error) {
	kid, err := h.rotateKeysHandler.Handle(ctx, command.RotateKeysCommand{
		KeyTTL: 7 * 24 * time.Hour,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to rotate keys: %v", err)
	}

	return &authv1message.RotateKeysResponse{
		Message: "Key rotation completed. New Key ID: " + kid,
	}, nil
}

// GetJWKS serves active and inactive keys in standard JWKS JSON payload format.
func (h *GRPCHandler) GetJWKS(
	ctx context.Context,
	_ *authv1message.GetJWKSRequest,
) (*authv1message.GetJWKSResponse, error) {
	jwks, err := h.getJWKSHandler.Handle(ctx, query.GetJWKSQuery{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get JWKS: %v", err)
	}

	protoKeys := make([]*authv1message.JWKKey, 0, len(jwks))
	for _, k := range jwks {
		protoKeys = append(protoKeys, &authv1message.JWKKey{
			Kty: k.Kty,
			Use: k.Use,
			Alg: k.Alg,
			Kid: k.Kid,
			N:   k.N,
			E:   k.E,
		})
	}

	return &authv1message.GetJWKSResponse{
		Keys: protoKeys,
	}, nil
}

// LoginWithOAuth processes OAuth codes for standard authentication.
func (h *GRPCHandler) LoginWithOAuth(
	ctx context.Context,
	req *authv1message.LoginWithOAuthRequest,
) (*authv1message.LoginResponse, error) {
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

	return &authv1message.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserId:       userID,
	}, nil
}
