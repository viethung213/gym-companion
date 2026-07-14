package port

import "context"

// OAuthUserProfile represents user profile information retrieved from Google/Facebook.
type OAuthUserProfile struct {
	ID        string
	Email     string
	FullName  string
	AvatarURL string
}

// OAuthService defines the application port for communicating with Google and Facebook OAuth APIs.
type OAuthService interface {
	GetOAuthLoginURL(ctx context.Context, provider string, state string, redirectURI string) (string, error)
	ExchangeCodeForProfile(ctx context.Context, provider string, code string, redirectURI string) (*OAuthUserProfile, error)
	GenerateState(ctx context.Context) (string, error)
	ValidateState(ctx context.Context, state string) error
}
