package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/repository"
)

// OAuthLoginCommand represents the command to log in/register via OAuth callback.
type OAuthLoginCommand struct {
	Provider    string
	Code        string
	RedirectURI string
	State       string
}

// OAuthLoginHandler exchanges OAuth authorization code for tokens and manages user accounts.
type OAuthLoginHandler struct {
	userRepo  repository.UserRepository
	keyRepo   port.KeyRepository
	sessRepo  port.SessionRepository
	tokenServ port.TokenService
	oauthServ port.OAuthService
	publisher port.OutboxWriter
	txManager port.TransactionManager
}

// NewOAuthLoginHandler creates a new OAuthLoginHandler instance.
func NewOAuthLoginHandler(
	userRepo repository.UserRepository,
	keyRepo port.KeyRepository,
	sessRepo port.SessionRepository,
	tokenServ port.TokenService,
	oauthServ port.OAuthService,
	publisher port.OutboxWriter,
	txManager port.TransactionManager,
) *OAuthLoginHandler {
	return &OAuthLoginHandler{
		userRepo:  userRepo,
		keyRepo:   keyRepo,
		sessRepo:  sessRepo,
		tokenServ: tokenServ,
		oauthServ: oauthServ,
		publisher: publisher,
		txManager: txManager,
	}
}

// Handle executes the OAuth callback exchanging and user linking/creation login flow.
func (h *OAuthLoginHandler) Handle(ctx context.Context, cmd OAuthLoginCommand) (string, string, string, error) {
	// Guard Clause: Only accept supported OAuth providers (Google and Facebook)
	if !aggregate.IsSupportedOAuthProvider(cmd.Provider) {
		return "", "", "", fmt.Errorf("unsupported oauth provider: %s", cmd.Provider)
	}

	// 1. Validate OAuth2 state to prevent CSRF attacks
	if err := h.oauthServ.ValidateState(ctx, cmd.State); err != nil {
		return "", "", "", fmt.Errorf("oauth state validation failed: %w", err)
	}

	// 2. Pre-check active signing key BEFORE exchanging code or modifying DB
	activeKey, err := h.keyRepo.GetActiveKey(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("active signing key not found: %w", err)
	}

	// 3. Exchange OAuth code for User Profile
	profile, err := h.oauthServ.ExchangeCodeForProfile(ctx, cmd.Provider, cmd.Code, cmd.RedirectURI)
	if err != nil {
		return "", "", "", fmt.Errorf("oauth exchange failed: %w", err)
	}

	// 4. Query user by Social Identity (GoogleID / FacebookID)
	user, err := h.userRepo.FindByIdentity(ctx, cmd.Provider, profile.ID)
	if err != nil && !errors.Is(err, derror.ErrUserNotFound) {
		return "", "", "", fmt.Errorf("find user by social id: %w", err)
	}

	var accessToken, refreshTokenStr string
	var expiresAt time.Time

	// 5. Execute User registration/linking, Token generation, and Session saving in a SINGLE Atomic Transaction
	err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if user == nil {
			if !profile.EmailVerified {
				return fmt.Errorf("cannot register social account: email %s is not verified by provider %s", profile.Email, cmd.Provider)
			}
			// Register new user with OAuth identity
			newUserID := uuid.New().String()
			now := time.Now()
			metaJSON, _ := json.Marshal(map[string]string{
				"email": profile.Email,
			})
			socialIdentity := aggregate.NewIdentity(
				uuid.New().String(),
				cmd.Provider,
				profile.ID,
				"",
				metaJSON,
				now,
				now,
			)

			user = aggregate.RegisterUser(
				newUserID,
				profile.FullName,
				socialIdentity,
				profile.AvatarURL,
			)

			if createErr := h.userRepo.Create(txCtx, user); createErr != nil {
				return fmt.Errorf("save new user: %w", createErr)
			}

			// Publish domain events
			for _, ev := range user.DomainEvents() {
				if pubErr := h.publisher.Write(txCtx, ev); pubErr != nil {
					return fmt.Errorf("dispatch domain event: %w", pubErr)
				}
			}
		}

		// Generate Access Token
		var tokenErr error
		accessToken, _, tokenErr = h.tokenServ.GenerateAccessToken(txCtx, user, activeKey.ID)
		if tokenErr != nil {
			return fmt.Errorf("generate access token: %w", tokenErr)
		}

		// Generate Refresh Token
		refreshTokenStr, expiresAt, tokenErr = h.tokenServ.GenerateRefreshToken(txCtx, user)
		if tokenErr != nil {
			return fmt.Errorf("generate refresh token: %w", tokenErr)
		}

		// Save Session
		if sessErr := h.sessRepo.Save(txCtx, refreshTokenStr, user.ID(), expiresAt); sessErr != nil {
			return fmt.Errorf("save session: %w", sessErr)
		}

		return nil
	})

	if err != nil {
		return "", "", "", err
	}

	if user != nil {
		user.ClearDomainEvents()
	}

	return accessToken, refreshTokenStr, user.ID(), nil
}
