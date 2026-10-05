package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
)

// CredentialsLoginCommand contains credentials for standard password-based authentication.
type CredentialsLoginCommand struct {
	Identifier string // Email address or Phone number
	Password   string
}

// CredentialsLoginResult contains authentication tokens.
type CredentialsLoginResult struct {
	AccessToken  string
	RefreshToken string
	UserID       string
}

// CredentialsLoginHandler manages traditional password authentication, decoupled from OAuth.
type CredentialsLoginHandler struct {
	userRepo     repository.UserRepository
	hasher       port.Hasher
	keyRepo      port.KeyRepository
	tokenService port.TokenService
	sessionRepo  port.SessionRepository
}

// NewCredentialsLoginHandler creates a new CredentialsLoginHandler.
func NewCredentialsLoginHandler(
	userRepo repository.UserRepository,
	hasher port.Hasher,
	keyRepo port.KeyRepository,
	tokenService port.TokenService,
	sessionRepo port.SessionRepository,
) *CredentialsLoginHandler {
	return &CredentialsLoginHandler{
		userRepo:     userRepo,
		hasher:       hasher,
		keyRepo:      keyRepo,
		tokenService: tokenService,
		sessionRepo:  sessionRepo,
	}
}

// Handle authenticates user using email/phone and password, returning JWT access & refresh tokens.
func (h *CredentialsLoginHandler) Handle(ctx context.Context, cmd CredentialsLoginCommand) (*CredentialsLoginResult, error) {
	rawIdentifier := strings.TrimSpace(cmd.Identifier)
	if rawIdentifier == "" {
		return nil, fmt.Errorf("identifier cannot be empty")
	}
	if cmd.Password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}

	// Tự động nhận diện loại identifier (Email hoặc Phone)
	var identityType, identifier string
	if strings.Contains(rawIdentifier, "@") {
		emailVO, err := vo.NewEmail(rawIdentifier)
		if err != nil {
			return nil, derror.ErrUnauthorized
		}
		identityType = aggregate.IdentityTypeEmail
		identifier = emailVO.Value()
	} else {
		phoneVO, err := vo.NewPhone(rawIdentifier)
		if err != nil {
			return nil, derror.ErrUnauthorized
		}
		identityType = aggregate.IdentityTypePhone
		identifier = phoneVO.Value()
	}

	user, err := h.userRepo.FindByIdentity(ctx, identityType, identifier)
	if err != nil {
		return nil, derror.ErrUnauthorized
	}

	if user.Status() == aggregate.UserStatusLocked {
		return nil, derror.ErrUserLocked
	}
	if user.Status() == aggregate.UserStatusSuspended {
		return nil, derror.ErrUserSuspended
	}
	if user.Status() != aggregate.UserStatusActive {
		return nil, derror.ErrUnauthorized
	}

	// Đối chiếu mật khẩu
	if err := h.hasher.Compare(user.Identity().CredentialData(), cmd.Password); err != nil {
		return nil, derror.ErrUnauthorized
	}

	// Lấy Active Key để ký JWT
	activeKey, err := h.keyRepo.GetActiveKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieve active key: %w", err)
	}

	// Ký Access Token
	accessToken, _, err := h.tokenService.GenerateAccessToken(ctx, user, activeKey.ID)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Tạo Refresh Token
	refreshToken, expiresAt, err := h.tokenService.GenerateRefreshToken(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	// Lưu phiên làm việc
	if err := h.sessionRepo.Save(ctx, refreshToken, user.ID(), expiresAt); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return &CredentialsLoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID(),
	}, nil
}
