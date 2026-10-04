package grpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/viethung213/gym-companion/internal/auth/application/apperror"
	"github.com/viethung213/gym-companion/internal/auth/application/command"
	"github.com/viethung213/gym-companion/internal/auth/application/query"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
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
	oauthLoginHandler        *command.OAuthLoginHandler
	logoutHandler            *command.LogoutHandler
	rotateKeysHandler        *command.RotateKeysHandler
	refreshTokenHandler      *command.RefreshTokenHandler
	getJWKSHandler           *query.GetJWKSHandler
	getOAuthLoginURLHandler  *query.GetOAuthLoginURLHandler
	registerHandler          *command.RegisterHandler
	credentialsLoginHandler  *command.CredentialsLoginHandler
	sendOTPHandler           *command.SendOTPHandler
	verifyOTPHandler         *command.VerifyOTPHandler
	changePasswordHandler    *command.ChangePasswordHandler
	resetPasswordHandler     *command.ResetPasswordHandler
	registerBrandHandler     *command.RegisterBrandHandler
	approveBrandHandler      *command.ApproveBrandHandler
	rejectBrandHandler       *command.RejectBrandHandler
	listBrandRequestsHandler *query.ListBrandRequestsHandler
	listUsersHandler         *query.ListUsersHandler
	lockUserHandler          *command.LockUserHandler
	unlockUserHandler        *command.UnlockUserHandler
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
	registerBrandHandler *command.RegisterBrandHandler,
	approveBrandHandler *command.ApproveBrandHandler,
	rejectBrandHandler *command.RejectBrandHandler,
	listBrandRequestsHandler *query.ListBrandRequestsHandler,
	listUsersHandler *query.ListUsersHandler,
	lockUserHandler *command.LockUserHandler,
	unlockUserHandler *command.UnlockUserHandler,
) *GRPCHandler {
	return &GRPCHandler{
		oauthLoginHandler:        oauthLoginHandler,
		logoutHandler:            logoutHandler,
		rotateKeysHandler:        rotateKeysHandler,
		refreshTokenHandler:      refreshTokenHandler,
		getJWKSHandler:           getJWKSHandler,
		getOAuthLoginURLHandler:  getOAuthLoginURLHandler,
		registerHandler:          registerHandler,
		credentialsLoginHandler:  credentialsLoginHandler,
		sendOTPHandler:           sendOTPHandler,
		verifyOTPHandler:         verifyOTPHandler,
		changePasswordHandler:    changePasswordHandler,
		resetPasswordHandler:     resetPasswordHandler,
		registerBrandHandler:     registerBrandHandler,
		approveBrandHandler:      approveBrandHandler,
		rejectBrandHandler:       rejectBrandHandler,
		listBrandRequestsHandler: listBrandRequestsHandler,
		listUsersHandler:         listUsersHandler,
		lockUserHandler:          lockUserHandler,
		unlockUserHandler:        unlockUserHandler,
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
		if errors.Is(err, derror.ErrUserLocked) {
			return nil, status.Errorf(codes.PermissionDenied, "tài khoản của bạn đã bị khóa, vui lòng liên hệ quản trị viên")
		}
		if errors.Is(err, derror.ErrUserSuspended) {
			return nil, status.Errorf(codes.PermissionDenied, "tài khoản của bạn đã bị tạm đình chỉ, vui lòng liên hệ quản trị viên")
		}
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
		if errors.Is(err, derror.ErrUserLocked) {
			return nil, status.Errorf(codes.PermissionDenied, "tài khoản của bạn đã bị khóa, vui lòng liên hệ quản trị viên")
		}
		if errors.Is(err, derror.ErrUserSuspended) {
			return nil, status.Errorf(codes.PermissionDenied, "tài khoản của bạn đã bị tạm đình chỉ, vui lòng liên hệ quản trị viên")
		}
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
		if errors.Is(err, derror.ErrUserLocked) {
			return nil, status.Errorf(codes.PermissionDenied, "tài khoản của bạn đã bị khóa, vui lòng liên hệ quản trị viên")
		}
		if errors.Is(err, derror.ErrUserSuspended) {
			return nil, status.Errorf(codes.PermissionDenied, "tài khoản của bạn đã bị tạm đình chỉ, vui lòng liên hệ quản trị viên")
		}
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

// RegisterBrand submits an upgrade request from an authenticated user to become a Brand.
func (h *GRPCHandler) RegisterBrand(
	ctx context.Context,
	req *authv1message.RegisterBrandRequest,
) (*authv1message.RegisterBrandResponse, error) {
	if h.registerBrandHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method RegisterBrand not implemented")
	}

	userID := extractUserID(ctx)
	if userID == "" {
		return nil, status.Errorf(codes.Unauthenticated, "yêu cầu đăng nhập")
	}

	_, err := h.registerBrandHandler.Handle(ctx, command.RegisterBrandCommand{
		UserID:       userID,
		BrandName:    req.BrandName,
		Description:  req.Description,
		ContactPhone: req.ContactPhone,
		Address:      req.Address,
	})
	if err != nil {
		if errors.Is(err, derror.ErrUnauthorized) {
			return nil, status.Errorf(codes.Unauthenticated, "yêu cầu đăng nhập")
		}
		if errors.Is(err, derror.ErrUserAlreadyBrand) {
			return nil, status.Errorf(codes.FailedPrecondition, "người dùng đã là Brand")
		}
		if errors.Is(err, derror.ErrBrandRequestAlreadyPending) {
			return nil, status.Errorf(codes.AlreadyExists, "bạn đã có một yêu cầu đăng ký Brand đang chờ xét duyệt")
		}
		if errors.Is(err, derror.ErrInvalidBrandRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "thông tin đăng ký Brand không hợp lệ: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "đăng ký Brand thất bại: %v", err)
	}

	return &authv1message.RegisterBrandResponse{
		Success: true,
		Message: "Gửi yêu cầu đăng ký Brand thành công, vui lòng chờ Admin phê duyệt",
	}, nil
}

// ApproveBrand approves a pending brand registration request (Admin only).
func (h *GRPCHandler) ApproveBrand(
	ctx context.Context,
	req *authv1message.ApproveBrandRequest,
) (*authv1message.ApproveBrandResponse, error) {
	if h.approveBrandHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method ApproveBrand not implemented")
	}

	actor, err := extractActor(ctx)
	if err != nil || !actor.IsAdmin() {
		return nil, status.Errorf(codes.PermissionDenied, "chỉ Quản trị viên (Admin) mới có quyền phê duyệt")
	}

	err = h.approveBrandHandler.Handle(ctx, command.ApproveBrandCommand{
		AdminID:   actor.UserID,
		RequestID: req.RequestId,
	})
	if err != nil {
		if errors.Is(err, derror.ErrBrandRequestNotFound) {
			return nil, status.Errorf(codes.NotFound, "không tìm thấy yêu cầu đăng ký Brand")
		}
		if errors.Is(err, derror.ErrBrandRequestNotPending) {
			return nil, status.Errorf(codes.FailedPrecondition, "yêu cầu không ở trạng thái chờ duyệt (pending)")
		}
		return nil, status.Errorf(codes.Internal, "phê duyệt yêu cầu thất bại: %v", err)
	}

	return &authv1message.ApproveBrandResponse{
		Success: true,
		Message: "Yêu cầu đăng ký Brand đã được phê duyệt thành công",
	}, nil
}

// RejectBrand rejects a pending brand registration request with a reason (Admin only).
func (h *GRPCHandler) RejectBrand(
	ctx context.Context,
	req *authv1message.RejectBrandRequest,
) (*authv1message.RejectBrandResponse, error) {
	if h.rejectBrandHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method RejectBrand not implemented")
	}

	actor, err := extractActor(ctx)
	if err != nil || !actor.IsAdmin() {
		return nil, status.Errorf(codes.PermissionDenied, "chỉ Quản trị viên (Admin) mới có quyền từ chối")
	}

	err = h.rejectBrandHandler.Handle(ctx, command.RejectBrandCommand{
		AdminID:   actor.UserID,
		RequestID: req.RequestId,
		Reason:    req.Reason,
	})
	if err != nil {
		if errors.Is(err, derror.ErrBrandRequestNotFound) {
			return nil, status.Errorf(codes.NotFound, "không tìm thấy yêu cầu đăng ký Brand")
		}
		if errors.Is(err, derror.ErrBrandRequestNotPending) {
			return nil, status.Errorf(codes.FailedPrecondition, "yêu cầu không ở trạng thái chờ duyệt (pending)")
		}
		return nil, status.Errorf(codes.Internal, "từ chối yêu cầu thất bại: %v", err)
	}

	return &authv1message.RejectBrandResponse{
		Success: true,
		Message: "Yêu cầu đăng ký Brand đã bị từ chối",
	}, nil
}

// ListBrandRequests lists brand requests with pagination and status filtering (Admin only).
func (h *GRPCHandler) ListBrandRequests(
	ctx context.Context,
	req *authv1message.ListBrandRequestsRequest,
) (*authv1message.ListBrandRequestsResponse, error) {
	if h.listBrandRequestsHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method ListBrandRequests not implemented")
	}

	actor, err := extractActor(ctx)
	if err != nil || !actor.IsAdmin() {
		return nil, status.Errorf(codes.PermissionDenied, "chỉ Quản trị viên (Admin) mới có quyền xem danh sách yêu cầu")
	}

	var statusFilter string
	switch req.Status {
	case authv1message.BrandRequestStatus_BRAND_REQUEST_STATUS_PENDING:
		statusFilter = "pending"
	case authv1message.BrandRequestStatus_BRAND_REQUEST_STATUS_APPROVED:
		statusFilter = "approved"
	case authv1message.BrandRequestStatus_BRAND_REQUEST_STATUS_REJECTED:
		statusFilter = "rejected"
	}

	res, err := h.listBrandRequestsHandler.Handle(ctx, query.ListBrandRequestsQuery{
		Status:   statusFilter,
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "lấy danh sách yêu cầu thất bại: %v", err)
	}

	protoItems := make([]*authv1message.BrandRequest, len(res.Items))
	for i, item := range res.Items {
		protoItems[i] = toProtoBrandRequest(item)
	}

	return &authv1message.ListBrandRequestsResponse{
		Items:    protoItems,
		Total:    int32(res.Total),
		Page:     int32(res.Page),
		PageSize: int32(res.PageSize),
	}, nil
}

// ListUsers lists users with filtering by role, status, search keyword, and pagination (Admin only).
func (h *GRPCHandler) ListUsers(
	ctx context.Context,
	req *authv1message.ListUsersRequest,
) (*authv1message.ListUsersResponse, error) {
	if h.listUsersHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method ListUsers not implemented")
	}

	actor, err := extractActor(ctx)
	if err != nil || !actor.IsAdmin() {
		return nil, status.Errorf(codes.PermissionDenied, "chỉ Quản trị viên (Admin) mới có quyền xem danh sách người dùng")
	}

	res, err := h.listUsersHandler.Handle(ctx, query.ListUsersQuery{
		Role:     req.Role,
		Status:   req.Status,
		Search:   req.Search,
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "lấy danh sách người dùng thất bại: %v", err)
	}

	protoItems := make([]*authv1message.UserInfo, len(res.Items))
	for i, u := range res.Items {
		protoItems[i] = toProtoUserInfo(u)
	}

	return &authv1message.ListUsersResponse{
		Items:    protoItems,
		Total:    int32(res.Total),
		Page:     int32(res.Page),
		PageSize: int32(res.PageSize),
	}, nil
}

// LockUser locks a user account (Admin only).
func (h *GRPCHandler) LockUser(
	ctx context.Context,
	req *authv1message.LockUserRequest,
) (*authv1message.LockUserResponse, error) {
	if h.lockUserHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method LockUser not implemented")
	}

	actor, err := extractActor(ctx)
	if err != nil || !actor.IsAdmin() {
		return nil, status.Errorf(codes.PermissionDenied, "chỉ Quản trị viên (Admin) mới có quyền khóa tài khoản")
	}

	err = h.lockUserHandler.Handle(ctx, command.LockUserCommand{
		AdminID: actor.UserID,
		UserID:  req.UserId,
		Reason:  req.Reason,
	})
	if err != nil {
		if errors.Is(err, derror.ErrUserNotFound) {
			return nil, status.Errorf(codes.NotFound, "không tìm thấy người dùng")
		}
		if errors.Is(err, derror.ErrCannotLockAdmin) {
			return nil, status.Errorf(codes.PermissionDenied, "không thể khóa tài khoản Quản trị viên")
		}
		if errors.Is(err, derror.ErrUserAlreadyLocked) {
			return nil, status.Errorf(codes.FailedPrecondition, "tài khoản người dùng đã ở trạng thái bị khóa")
		}
		return nil, status.Errorf(codes.Internal, "khóa tài khoản thất bại: %v", err)
	}

	return &authv1message.LockUserResponse{
		Success: true,
		Message: "Khóa tài khoản người dùng thành công",
	}, nil
}

// UnlockUser unlocks a user account (Admin only).
func (h *GRPCHandler) UnlockUser(
	ctx context.Context,
	req *authv1message.UnlockUserRequest,
) (*authv1message.UnlockUserResponse, error) {
	if h.unlockUserHandler == nil {
		return nil, status.Errorf(codes.Unimplemented, "method UnlockUser not implemented")
	}

	actor, err := extractActor(ctx)
	if err != nil || !actor.IsAdmin() {
		return nil, status.Errorf(codes.PermissionDenied, "chỉ Quản trị viên (Admin) mới có quyền mở khóa tài khoản")
	}

	err = h.unlockUserHandler.Handle(ctx, command.UnlockUserCommand{
		AdminID: actor.UserID,
		UserID:  req.UserId,
	})
	if err != nil {
		if errors.Is(err, derror.ErrUserNotFound) {
			return nil, status.Errorf(codes.NotFound, "không tìm thấy người dùng")
		}
		if errors.Is(err, derror.ErrUserAlreadyActive) {
			return nil, status.Errorf(codes.FailedPrecondition, "tài khoản người dùng đang ở trạng thái hoạt động")
		}
		return nil, status.Errorf(codes.Internal, "mở khóa tài khoản thất bại: %v", err)
	}

	return &authv1message.UnlockUserResponse{
		Success: true,
		Message: "Mở khóa tài khoản người dùng thành công",
	}, nil
}

func toProtoUserInfo(u *aggregate.User) *authv1message.UserInfo {
	if u == nil {
		return nil
	}
	return &authv1message.UserInfo{
		Id:           u.ID(),
		FullName:     u.FullName(),
		Role:         u.Role(),
		Status:       u.Status(),
		IdentityType: u.Identity().IdentityType(),
		Identifier:   u.Identity().Identifier(),
		CreatedAt:    u.CreatedAt().Format(time.RFC3339),
		UpdatedAt:    u.UpdatedAt().Format(time.RFC3339),
	}
}

func extractUserID(ctx context.Context) string {
	userID, _ := ctx.Value(middleware.UserIDKey).(string)
	if userID != "" {
		return userID
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-user-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func extractActor(ctx context.Context) (middleware.Actor, error) {
	actor, err := middleware.RequireAuthenticated(ctx)
	if err == nil && actor.UserID != "" {
		return actor, nil
	}
	// Fallback to incoming metadata
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		userID := ""
		if vals := md.Get("x-user-id"); len(vals) > 0 {
			userID = vals[0]
		}
		role := ""
		if vals := md.Get("x-user-role"); len(vals) > 0 {
			role = vals[0]
		}
		if userID != "" {
			return middleware.Actor{
				UserID: userID,
				Role:   role,
			}, nil
		}
	}
	return middleware.Actor{}, middleware.ErrUnauthorized
}

func toProtoBrandRequest(r *entity.BrandRequest) *authv1message.BrandRequest {
	if r == nil {
		return nil
	}
	var statusEnum authv1message.BrandRequestStatus
	switch r.Status() {
	case entity.BrandRequestStatusPending:
		statusEnum = authv1message.BrandRequestStatus_BRAND_REQUEST_STATUS_PENDING
	case entity.BrandRequestStatusApproved:
		statusEnum = authv1message.BrandRequestStatus_BRAND_REQUEST_STATUS_APPROVED
	case entity.BrandRequestStatusRejected:
		statusEnum = authv1message.BrandRequestStatus_BRAND_REQUEST_STATUS_REJECTED
	default:
		statusEnum = authv1message.BrandRequestStatus_BRAND_REQUEST_STATUS_UNSPECIFIED
	}

	var reviewedAtStr string
	if r.ReviewedAt() != nil {
		reviewedAtStr = r.ReviewedAt().Format(time.RFC3339)
	}

	return &authv1message.BrandRequest{
		Id:              r.ID(),
		UserId:          r.UserID(),
		BrandName:       r.BrandName(),
		Description:     r.Description(),
		ContactPhone:    r.ContactPhone(),
		Address:         r.Address(),
		Status:          statusEnum,
		RejectionReason: r.RejectionReason(),
		ReviewedBy:      r.ReviewedBy(),
		ReviewedAt:      reviewedAtStr,
		CreatedAt:       r.CreatedAt().Format(time.RFC3339),
		UpdatedAt:       r.UpdatedAt().Format(time.RFC3339),
	}
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

func (c *ConnectAuthHandler) RegisterBrand(
	ctx context.Context,
	req *connect.Request[authv1message.RegisterBrandRequest],
) (*connect.Response[authv1message.RegisterBrandResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" && ctx.Value(middleware.UserIDKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	res, err := c.grpcHandler.RegisterBrand(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) ApproveBrand(
	ctx context.Context,
	req *connect.Request[authv1message.ApproveBrandRequest],
) (*connect.Response[authv1message.ApproveBrandResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" && ctx.Value(middleware.UserIDKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	if role := req.Header().Get("X-User-Role"); role != "" && ctx.Value(middleware.UserRoleKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserRoleKey, role)
	}
	res, err := c.grpcHandler.ApproveBrand(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) RejectBrand(
	ctx context.Context,
	req *connect.Request[authv1message.RejectBrandRequest],
) (*connect.Response[authv1message.RejectBrandResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" && ctx.Value(middleware.UserIDKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	if role := req.Header().Get("X-User-Role"); role != "" && ctx.Value(middleware.UserRoleKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserRoleKey, role)
	}
	res, err := c.grpcHandler.RejectBrand(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) ListBrandRequests(
	ctx context.Context,
	req *connect.Request[authv1message.ListBrandRequestsRequest],
) (*connect.Response[authv1message.ListBrandRequestsResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" && ctx.Value(middleware.UserIDKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	if role := req.Header().Get("X-User-Role"); role != "" && ctx.Value(middleware.UserRoleKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserRoleKey, role)
	}
	res, err := c.grpcHandler.ListBrandRequests(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) ListUsers(
	ctx context.Context,
	req *connect.Request[authv1message.ListUsersRequest],
) (*connect.Response[authv1message.ListUsersResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" && ctx.Value(middleware.UserIDKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	if role := req.Header().Get("X-User-Role"); role != "" && ctx.Value(middleware.UserRoleKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserRoleKey, role)
	}
	res, err := c.grpcHandler.ListUsers(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) LockUser(
	ctx context.Context,
	req *connect.Request[authv1message.LockUserRequest],
) (*connect.Response[authv1message.LockUserResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" && ctx.Value(middleware.UserIDKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	if role := req.Header().Get("X-User-Role"); role != "" && ctx.Value(middleware.UserRoleKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserRoleKey, role)
	}
	res, err := c.grpcHandler.LockUser(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (c *ConnectAuthHandler) UnlockUser(
	ctx context.Context,
	req *connect.Request[authv1message.UnlockUserRequest],
) (*connect.Response[authv1message.UnlockUserResponse], error) {
	if userID := req.Header().Get("X-User-Id"); userID != "" && ctx.Value(middleware.UserIDKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	}
	if role := req.Header().Get("X-User-Role"); role != "" && ctx.Value(middleware.UserRoleKey) == nil {
		ctx = context.WithValue(ctx, middleware.UserRoleKey, role)
	}
	res, err := c.grpcHandler.UnlockUser(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}
