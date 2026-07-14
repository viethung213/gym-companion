package query

import (
	"context"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// GetOAuthLoginURLQuery represents the query to retrieve the OAuth provider redirect url.
type GetOAuthLoginURLQuery struct {
	Provider    string
	RedirectURI string
}

// GetOAuthLoginURLHandler handles OAuth URL generation requests.
type GetOAuthLoginURLHandler struct {
	oauthServ port.OAuthService
}

// NewGetOAuthLoginURLHandler constructs a GetOAuthLoginURLHandler instance.
func NewGetOAuthLoginURLHandler(oauthServ port.OAuthService) *GetOAuthLoginURLHandler {
	return &GetOAuthLoginURLHandler{oauthServ: oauthServ}
}

// Handle generates the OAuth provider redirection login URL.
func (h *GetOAuthLoginURLHandler) Handle(ctx context.Context, query GetOAuthLoginURLQuery) (string, error) {
	state, err := h.oauthServ.GenerateState(ctx)
	if err != nil {
		return "", err
	}
	return h.oauthServ.GetOAuthLoginURL(ctx, query.Provider, state, query.RedirectURI)
}
