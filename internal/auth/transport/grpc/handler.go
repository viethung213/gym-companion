package grpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/command"
	"github.com/viethung213/gym-companion/internal/auth/application/query"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
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
	registerHandler         *command.RegisterHandler
	credentialsLoginHandler *command.CredentialsLoginHandler
	sendOTPHandler          *command.SendOTPHandler
	verifyOTPHandler        *command.VerifyOTPHandler
	changePasswordHandler   *command.ChangePasswordHandler
	resetPasswordHandler    *command.ResetPasswordHandler
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
	registerHandler *command.RegisterHandler,
	credentialsLoginHandler *command.CredentialsLoginHandler,
	sendOTPHandler *command.SendOTPHandler,
	verifyOTPHandler *command.VerifyOTPHandler,
	changePasswordHandler *command.ChangePasswordHandler,
	resetPasswordHandler *command.ResetPasswordHandler,
) *GRPCHandler {
	return &GRPCHandler{
		oauthLoginHandler:       oauthLoginHandler,
		logoutHandler:           logoutHandler,
		rotateKeysHandler:       rotateKeysHandler,
		refreshTokenHandler:     refreshTokenHandler,
		getJWKSHandler:          getJWKSHandler,
		getOAuthLoginURLHandler: getOAuthLoginURLHandler,
		registerHandler:         registerHandler,
		credentialsLoginHandler: credentialsLoginHandler,
		sendOTPHandler:          sendOTPHandler,
		verifyOTPHandler:        verifyOTPHandler,
		changePasswordHandler:   changePasswordHandler,
		resetPasswordHandler:    resetPasswordHandler,
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

// Register registers a new account using Email or Phone.
func (h *GRPCHandler) Register(
	ctx context.Context,
	req *authv1message.RegisterRequest,
) (*authv1message.RegisterResponse, error) {
	if h.registerHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method Register not implemented")
	}
	res, err := h.registerHandler.Handle(ctx, command.RegisterCommand{
		Identifier:  req.Identifier,
		Password:    req.Password,
		FullName:    req.FullName,
		Gender:      req.Gender,
		DateOfBirth: req.DateOfBirth,
	})
	if err != nil {
		if errors.Is(err, derror.ErrConflict) {
			return nil, status.Errorf(codes.AlreadyExists, "tài khoản đã tồn tại: %v", err)
		}
		return nil, status.Errorf(codes.InvalidArgument, "đăng ký thất bại: %v", err)
	}
	return &authv1message.RegisterResponse{Message: res.Message}, nil
}

// Login authenticates with email/phone and password.
func (h *GRPCHandler) Login(
	ctx context.Context,
	req *authv1message.LoginRequest,
) (*authv1message.LoginResponse, error) {
	if h.credentialsLoginHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method Login not implemented")
	}
	res, err := h.credentialsLoginHandler.Handle(ctx, command.CredentialsLoginCommand{
		Identifier: req.Identifier,
		Password:   req.Password,
	})
	if err != nil {
		if errors.Is(err, derror.ErrUnauthorized) {
			return nil, status.Errorf(codes.Unauthenticated, "tài khoản hoặc mật khẩu không chính xác")
		}
		return nil, status.Errorf(codes.Internal, "đăng nhập thất bại: %v", err)
	}
	return &authv1message.LoginResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		UserId:       res.UserID,
	}, nil
}

// SendOTP requests generation of an OTP code.
func (h *GRPCHandler) SendOTP(
	ctx context.Context,
	req *authv1message.SendOTPRequest,
) (*authv1message.SendOTPResponse, error) {
	if h.sendOTPHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method SendOTP not implemented")
	}
	res, err := h.sendOTPHandler.Handle(ctx, command.SendOTPCommand{
		Identifier: req.Identifier,
		Purpose:    req.Purpose,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "gửi OTP thất bại: %v", err)
	}
	return &authv1message.SendOTPResponse{
		Success:          res.Success,
		Message:          res.Message,
		ExpiresInSeconds: res.ExpiresInSeconds,
		OtpToken:         res.OTPToken,
	}, nil
}

// VerifyOTP verifies an OTP code using otp_token.
func (h *GRPCHandler) VerifyOTP(
	ctx context.Context,
	req *authv1message.VerifyOTPRequest,
) (*authv1message.VerifyOTPResponse, error) {
	if h.verifyOTPHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method VerifyOTP not implemented")
	}
	res, err := h.verifyOTPHandler.Handle(ctx, command.VerifyOTPCommand{
		OTPToken: req.OtpToken,
		Code:     req.Code,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "xác thực OTP thất bại: %v", err)
	}
	return &authv1message.VerifyOTPResponse{
		Valid:      res.Valid,
		ResetToken: res.ResetToken,
		Message:    res.Message,
	}, nil
}

// ChangePassword changes the user password for authenticated clients.
func (h *GRPCHandler) ChangePassword(
	ctx context.Context,
	req *authv1message.ChangePasswordRequest,
) (*authv1message.ChangePasswordResponse, error) {
	if h.changePasswordHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method ChangePassword not implemented")
	}
	userID, _ := ctx.Value(middleware.UserIDKey).(string)
	if userID == "" {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-user-id"); len(vals) > 0 {
				userID = vals[0]
			}
		}
	}
	res, err := h.changePasswordHandler.Handle(ctx, command.ChangePasswordCommand{
		UserID:          userID,
		OldPassword:     req.OldPassword,
		NewPassword:     req.NewPassword,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		if errors.Is(err, derror.ErrUnauthorized) {
			return nil, status.Errorf(codes.Unauthenticated, "mật khẩu cũ không chính xác")
		}
		return nil, status.Errorf(codes.InvalidArgument, "đổi mật khẩu thất bại: %v", err)
	}
	return &authv1message.ChangePasswordResponse{
		Success: res.Success,
		Message: res.Message,
	}, nil
}

// ResetPassword resets password after OTP verification using a reset_token.
func (h *GRPCHandler) ResetPassword(
	ctx context.Context,
	req *authv1message.ResetPasswordRequest,
) (*authv1message.ResetPasswordResponse, error) {
	if h.resetPasswordHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method ResetPassword not implemented")
	}
	res, err := h.resetPasswordHandler.Handle(ctx, command.ResetPasswordCommand{
		ResetToken:      req.ResetToken,
		NewPassword:     req.NewPassword,
		ConfirmPassword: req.ConfirmPassword,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "đặt lại mật khẩu thất bại: %v", err)
	}
	return &authv1message.ResetPasswordResponse{
		Success: res.Success,
		Message: res.Message,
	}, nil
}

// --- ConnectRPC Adapter ---

type ConnectAuthHandler struct {
	authv1serviceconnect.UnimplementedAuthServiceHandler
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

func (c *ConnectAuthHandler) Register(
	ctx context.Context,
	req *connect.Request[authv1message.RegisterRequest],
) (*connect.Response[authv1message.RegisterResponse], error) {
	res, err := c.grpcHandler.Register(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) Login(
	ctx context.Context,
	req *connect.Request[authv1message.LoginRequest],
) (*connect.Response[authv1message.LoginResponse], error) {
	res, err := c.grpcHandler.Login(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) SendOTP(
	ctx context.Context,
	req *connect.Request[authv1message.SendOTPRequest],
) (*connect.Response[authv1message.SendOTPResponse], error) {
	res, err := c.grpcHandler.SendOTP(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) VerifyOTP(
	ctx context.Context,
	req *connect.Request[authv1message.VerifyOTPRequest],
) (*connect.Response[authv1message.VerifyOTPResponse], error) {
	res, err := c.grpcHandler.VerifyOTP(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) ChangePassword(
	ctx context.Context,
	req *connect.Request[authv1message.ChangePasswordRequest],
) (*connect.Response[authv1message.ChangePasswordResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	res, err := c.grpcHandler.ChangePassword(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) ResetPassword(
	ctx context.Context,
	req *connect.Request[authv1message.ResetPasswordRequest],
) (*connect.Response[authv1message.ResetPasswordResponse], error) {
	res, err := c.grpcHandler.ResetPassword(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
