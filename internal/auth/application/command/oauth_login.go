package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
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
	txManager application.TransactionManager
}

// NewOAuthLoginHandler creates a new OAuthLoginHandler instance.
func NewOAuthLoginHandler(
	userRepo repository.UserRepository,
	keyRepo port.KeyRepository,
	sessRepo port.SessionRepository,
	tokenServ port.TokenService,
	oauthServ port.OAuthService,
	publisher port.OutboxWriter,
	txManager application.TransactionManager,
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
	// Guard Clause: Only accept Google and Facebook
	if cmd.Provider != "google" && cmd.Provider != "facebook" {
		return "", "", "", fmt.Errorf("unsupported oauth provider: %s", cmd.Provider)
	}

	// Validate OAuth2 state to prevent CSRF attacks
	if err := h.oauthServ.ValidateState(ctx, cmd.State); err != nil {
		return "", "", "", fmt.Errorf("oauth state validation failed: %w", err)
	}

	// 1. Exchange OAuth code for User Profile
	profile, err := h.oauthServ.ExchangeCodeForProfile(ctx, cmd.Provider, cmd.Code, cmd.RedirectURI)
	if err != nil {
		return "", "", "", fmt.Errorf("oauth exchange failed: %w", err)
	}

	// 2. Query user by GoogleID / FacebookID / Email
	var user *aggregate.User
	if cmd.Provider == "google" {
		user, err = h.userRepo.FindByGoogleID(ctx, profile.ID)
	} else if cmd.Provider == "facebook" {
		user, err = h.userRepo.FindByFacebookID(ctx, profile.ID)
	}

	// 3. Register user or link account inside database transaction
	if err != nil || user == nil {
		// Try to find user by email to link social provider
		user, err = h.userRepo.FindByEmail(ctx, profile.Email)
		if err == nil && user != nil {
			// Link account
			if cmd.Provider == "google" {
				user.LinkGoogle(profile.ID)
			} else if cmd.Provider == "facebook" {
				user.LinkFacebook(profile.ID)
			}

			err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
				return h.userRepo.Update(txCtx, user)
			})
			if err != nil {
				return "", "", "", fmt.Errorf("link oauth account: %w", err)
			}
		} else {
			// Register new user
			err = h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
				var regErr error
				newUserID := uuid.New().String()
				user, regErr = aggregate.RegisterUser(
					newUserID,
					profile.Email,
					profile.FullName,
					"user", // default role
				)
				if regErr != nil {
					return fmt.Errorf("domain social user validation failed: %w", regErr)
				}

				if cmd.Provider == "google" {
					user.LinkGoogle(profile.ID)
				} else if cmd.Provider == "facebook" {
					user.LinkFacebook(profile.ID)
				}

				if err := h.userRepo.Create(txCtx, user); err != nil {
					return fmt.Errorf("save new user: %w", err)
				}

				// Publish domain events
				for _, ev := range user.DomainEvents() {
					if err := h.publisher.Write(txCtx, ev); err != nil {
						return fmt.Errorf("dispatch domain event: %w", err)
					}
				}

				return nil
			})
			if err != nil {
				return "", "", "", err
			}
			if user != nil {
				user.ClearDomainEvents()
			}
		}
	}

	// 4. Fetch active key for signing
	activeKey, err := h.keyRepo.GetActiveKey(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("active signing key not found: %w", err)
	}

	// 5. Generate Access Token
	accessToken, _, err := h.tokenServ.GenerateAccessToken(ctx, user, activeKey.ID)
	if err != nil {
		return "", "", "", fmt.Errorf("generate access token: %w", err)
	}

	// 6. Generate Refresh Token
	refreshTokenStr, expiresAt, err := h.tokenServ.GenerateRefreshToken(ctx, user)
	if err != nil {
		return "", "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	// 7. Save Session
	if err := h.sessRepo.Save(ctx, refreshTokenStr, user.ID(), expiresAt); err != nil {
		return "", "", "", fmt.Errorf("save session: %w", err)
	}

	return accessToken, refreshTokenStr, user.ID(), nil
}
