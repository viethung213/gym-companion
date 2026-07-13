package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type OAuthProvider struct {
	googleConfig   ProviderConfig
	facebookConfig ProviderConfig
	httpClient     *http.Client
	stateSecret    string
}

// Compile-time interface verification
var _ port.OAuthService = (*OAuthProvider)(nil)

// NewOAuthProvider creates a new instance of OAuthProvider.
func NewOAuthProvider(google, facebook ProviderConfig, stateSecret string) *OAuthProvider {
	return &OAuthProvider{
		googleConfig:   google,
		facebookConfig: facebook,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		stateSecret:    stateSecret,
	}
}

// GenerateState generates a cryptographically secure, signed state string.
func (p *OAuthProvider) GenerateState(ctx context.Context) (string, error) {
	return GenerateState(p.stateSecret, 10*time.Minute)
}

// ValidateState checks if the state string is valid and not expired.
func (p *OAuthProvider) ValidateState(ctx context.Context, state string) error {
	return ValidateState(state, p.stateSecret)
}

// GetOAuthLoginURL generates redirect URL to Google/Facebook OAuth flow.
func (p *OAuthProvider) GetOAuthLoginURL(ctx context.Context, provider string, state string, redirectURI string) (string, error) {
	switch provider {
	case "google":
		rURI := redirectURI
		if rURI == "" {
			rURI = p.googleConfig.RedirectURI
		}
		u := fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?"+
				"client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
			url.QueryEscape(p.googleConfig.ClientID),
			url.QueryEscape(rURI),
			url.QueryEscape("openid email profile"),
			url.QueryEscape(state),
		)
		return u, nil
	case "facebook":
		rURI := redirectURI
		if rURI == "" {
			rURI = p.facebookConfig.RedirectURI
		}
		u := fmt.Sprintf(
			"https://www.facebook.com/v12.0/dialog/oauth?"+
				"client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
			url.QueryEscape(p.facebookConfig.ClientID),
			url.QueryEscape(rURI),
			url.QueryEscape("email public_profile"),
			url.QueryEscape(state),
		)
		return u, nil
	}
	return "", fmt.Errorf("unknown oauth provider: %s", provider)
}

// ExchangeCodeForProfile exchanges authorization code for social profile data.
func (p *OAuthProvider) ExchangeCodeForProfile(ctx context.Context, provider string, code string, redirectURI string) (*port.OAuthUserProfile, error) {
	switch provider {
	case "google":
		return p.exchangeGoogle(ctx, code, redirectURI)
	case "facebook":
		return p.exchangeFacebook(ctx, code, redirectURI)
	}
	return nil, fmt.Errorf("unknown oauth provider: %s", provider)
}

func (p *OAuthProvider) exchangeGoogle(ctx context.Context, code string, redirectURI string) (*port.OAuthUserProfile, error) {
	rURI := redirectURI
	if rURI == "" {
		rURI = p.googleConfig.RedirectURI
	}

	// 1. Exchange code
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.googleConfig.ClientID)
	data.Set("client_secret", p.googleConfig.ClientSecret)
	data.Set("redirect_uri", rURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://oauth2.googleapis.com/token",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"token exchange failed with status %d: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	// 2. Fetch profile
	reqProf, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("create profile request: %w", err)
	}
	reqProf.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	respProf, err := p.httpClient.Do(reqProf)
	if err != nil {
		return nil, fmt.Errorf("execute profile request: %w", err)
	}
	defer respProf.Body.Close()

	if respProf.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respProf.Body)
		return nil, fmt.Errorf(
			"fetch profile failed with status %d: %s",
			respProf.StatusCode,
			string(bodyBytes),
		)
	}

	var profileResp struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(respProf.Body).Decode(&profileResp); err != nil {
		return nil, fmt.Errorf("decode profile response: %w", err)
	}

	return &port.OAuthUserProfile{
		ID:       profileResp.ID,
		Email:    profileResp.Email,
		FullName: profileResp.Name,
	}, nil
}

func (p *OAuthProvider) exchangeFacebook(ctx context.Context, code string, redirectURI string) (*port.OAuthUserProfile, error) {
	rURI := redirectURI
	if rURI == "" {
		rURI = p.facebookConfig.RedirectURI
	}

	// 1. Exchange code
	val := url.Values{}
	val.Set("code", code)
	val.Set("client_id", p.facebookConfig.ClientID)
	val.Set("client_secret", p.facebookConfig.ClientSecret)
	val.Set("redirect_uri", rURI)

	tokenURL := "https://graph.facebook.com/v12.0/oauth/access_token?" + val.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", tokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create facebook token request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute facebook token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"facebook token exchange failed with status %d: %s",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode facebook token response: %w", err)
	}

	// 2. Fetch profile
	valProf := url.Values{}
	valProf.Set("fields", "id,email,name,picture")
	valProf.Set("access_token", tokenResp.AccessToken)

	profileURL := "https://graph.facebook.com/me?" + valProf.Encode()
	reqProf, err := http.NewRequestWithContext(ctx, "GET", profileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create facebook profile request: %w", err)
	}

	respProf, err := p.httpClient.Do(reqProf)
	if err != nil {
		return nil, fmt.Errorf("execute facebook profile request: %w", err)
	}
	defer respProf.Body.Close()

	if respProf.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respProf.Body)
		return nil, fmt.Errorf(
			"facebook fetch profile failed with status %d: %s",
			respProf.StatusCode,
			string(bodyBytes),
		)
	}

	var profileResp struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := json.NewDecoder(respProf.Body).Decode(&profileResp); err != nil {
		return nil, fmt.Errorf("decode facebook profile response: %w", err)
	}

	return &port.OAuthUserProfile{
		ID:       profileResp.ID,
		Email:    profileResp.Email,
		FullName: profileResp.Name,
	}, nil
}
