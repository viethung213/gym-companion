//go:build e2e

package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	infraPostgres "github.com/viethung213/gym-companion/internal/auth/infrastructure/persistence/postgres"
)

// TestE2E_JWKS_And_KeyRotation tests public keys retrieval and manual rotation flows.
func TestE2E_JWKS_And_KeyRotation(t *testing.T) {
	baseURL, _, cleanup := startE2ETestServer(t)
	defer cleanup()

	// 1. Fetch JWKS Initially
	resp, err := http.Get(baseURL + "/api/v1/auth/jwks")
	if err != nil {
		t.Fatalf("Failed to fetch initial JWKS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Initial JWKS request failed with status %d", resp.StatusCode)
	}

	var initialJWKS struct {
		Keys []struct {
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&initialJWKS)
	if len(initialJWKS.Keys) != 1 {
		t.Errorf("Expected 1 initial key in JWKS, got %d", len(initialJWKS.Keys))
	}

	initialKid := initialJWKS.Keys[0].Kid

	// 2. Trigger Manual Key Rotation
	respRotate, err := http.Post(baseURL+"/api/v1/auth/keys/rotate", "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		t.Fatalf("Failed to execute key rotation request: %v", err)
	}
	defer respRotate.Body.Close()

	if respRotate.StatusCode != http.StatusOK {
		t.Fatalf("Key rotation failed with status %d", respRotate.StatusCode)
	}

	var rotateResp struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(respRotate.Body).Decode(&rotateResp)
	if rotateResp.Message == "" {
		t.Error("Expected key rotation response message, got empty")
	}

	// 3. Fetch JWKS after rotation
	respNew, err := http.Get(baseURL + "/api/v1/auth/jwks")
	if err != nil {
		t.Fatalf("Failed to fetch JWKS after rotation: %v", err)
	}
	defer respNew.Body.Close()

	var newJWKS struct {
		Keys []struct {
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	_ = json.NewDecoder(respNew.Body).Decode(&newJWKS)

	// Since rotation marks previous active key inactive, both should now appear in JWKS
	if len(newJWKS.Keys) < 2 {
		t.Errorf("Expected at least 2 keys after rotation, got %d", len(newJWKS.Keys))
	}

	foundInitial := false
	foundNew := false
	for _, k := range newJWKS.Keys {
		if k.Kid == initialKid {
			foundInitial = true
		} else {
			foundNew = true
		}
	}

	if !foundInitial {
		t.Errorf("Initial key %s was deleted or missing after rotation", initialKid)
	}
	if !foundNew {
		t.Error("No new active key was found in JWKS after rotation")
	}
}

// TestE2E_OAuthFlows_Google verifies Google login, token refresh, and logout E2E flows.
func TestE2E_OAuthFlows_Google(t *testing.T) {
	teardownMock := setupOAuthMock()
	defer teardownMock()

	baseURL, db, cleanup := startE2ETestServer(t)
	defer cleanup()

	// 1. Get OAuth Login URL for Google
	redirectURI := "http://localhost:3000/oauth/callback"
	loginURLPath := fmt.Sprintf("/api/v1/auth/oauth/login?provider=google&redirectUri=%s", url.QueryEscape(redirectURI))
	respURL, err := http.Get(baseURL + loginURLPath)
	if err != nil {
		t.Fatalf("Failed to execute GetOAuthLoginURL request: %v", err)
	}
	defer respURL.Body.Close()

	if respURL.StatusCode != http.StatusOK {
		t.Fatalf("GetOAuthLoginURL failed with status %d", respURL.StatusCode)
	}

	var urlResp struct {
		LoginUrl string `json:"loginUrl"`
	}
	_ = json.NewDecoder(respURL.Body).Decode(&urlResp)
	if urlResp.LoginUrl == "" {
		t.Fatal("Google login URL was empty")
	}

	parsedURL, err := url.Parse(urlResp.LoginUrl)
	if err != nil {
		t.Fatalf("Failed to parse Google login URL: %v", err)
	}
	stateVal := parsedURL.Query().Get("state")

	// 2. Perform LoginWithOAuth
	loginReq := map[string]interface{}{
		"provider":    "google",
		"code":        "google_auth_code_e2e",
		"redirectUri": redirectURI,
		"state":       stateVal,
	}
	reqBody, _ := json.Marshal(loginReq)

	respLogin, err := http.Post(baseURL+"/api/v1/auth/login/oauth", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to execute LoginWithOAuth request: %v", err)
	}
	defer respLogin.Body.Close()

	if respLogin.StatusCode != http.StatusOK {
		t.Fatalf("LoginWithOAuth failed with status %d", respLogin.StatusCode)
	}

	var loginResp struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		UserID       string `json:"userId"`
	}
	_ = json.NewDecoder(respLogin.Body).Decode(&loginResp)

	if loginResp.AccessToken == "" || loginResp.RefreshToken == "" || loginResp.UserID == "" {
		t.Fatalf("Login response returned empty values: %+v", loginResp)
	}

	// Verify DB state
	var user infraPostgres.UserModel
	if err := db.First(&user, "id = ?", loginResp.UserID).Error; err != nil {
		t.Fatalf("User record was not created: %v", err)
	}
	if user.Email != "google-e2e-user@example.com" {
		t.Errorf("Expected user email google-e2e-user@example.com, got %s", user.Email)
	}

	// 3. Perform RefreshToken
	refreshReq := map[string]interface{}{
		"refreshToken": loginResp.RefreshToken,
	}
	refreshBody, _ := json.Marshal(refreshReq)

	respRefresh, err := http.Post(baseURL+"/api/v1/auth/refresh", "application/json", bytes.NewBuffer(refreshBody))
	if err != nil {
		t.Fatalf("Failed to execute RefreshToken request: %v", err)
	}
	defer respRefresh.Body.Close()

	if respRefresh.StatusCode != http.StatusOK {
		t.Fatalf("RefreshToken failed with status %d", respRefresh.StatusCode)
	}

	var refreshResp struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(respRefresh.Body).Decode(&refreshResp)

	if refreshResp.AccessToken == "" || refreshResp.RefreshToken == "" {
		t.Fatalf("Refresh response returned empty values: %+v", refreshResp)
	}

	// 4. Perform Logout
	logoutReq := map[string]interface{}{
		"refreshToken": refreshResp.RefreshToken,
	}
	logoutBody, _ := json.Marshal(logoutReq)

	reqLogout, err := http.NewRequest("POST", baseURL+"/api/v1/auth/logout", bytes.NewBuffer(logoutBody))
	if err != nil {
		t.Fatalf("Failed to create Logout request: %v", err)
	}
	reqLogout.Header.Set("Content-Type", "application/json")
	reqLogout.Header.Set("X-User-Id", loginResp.UserID)
	reqLogout.Header.Set("Grpc-Metadata-X-User-Id", loginResp.UserID)

	respLogout, err := http.DefaultClient.Do(reqLogout)
	if err != nil {
		t.Fatalf("Failed to execute Logout request: %v", err)
	}
	defer respLogout.Body.Close()

	if respLogout.StatusCode != http.StatusOK {
		t.Fatalf("Logout failed with status %d", respLogout.StatusCode)
	}

	var logoutResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(respLogout.Body).Decode(&logoutResp)
	if !logoutResp.Success {
		t.Fatalf("Logout response success was false: %s", logoutResp.Message)
	}

	// Verify session has been deleted
	var sessionCount int64
	db.Model(&infraPostgres.SessionModel{}).Where("token = ?", hashToken(refreshResp.RefreshToken)).Count(&sessionCount)
	if sessionCount != 0 {
		t.Error("Expected session to be deleted from DB, but it still exists")
	}
}

// TestE2E_OAuthFlows_Facebook verifies Facebook login, token refresh, and logout E2E flows.
func TestE2E_OAuthFlows_Facebook(t *testing.T) {
	teardownMock := setupOAuthMock()
	defer teardownMock()

	baseURL, db, cleanup := startE2ETestServer(t)
	defer cleanup()

	// 1. Get OAuth Login URL for Facebook
	redirectURI := "http://localhost:3000/oauth/callback"
	loginURLPath := fmt.Sprintf("/api/v1/auth/oauth/login?provider=facebook&redirectUri=%s", url.QueryEscape(redirectURI))
	respURL, err := http.Get(baseURL + loginURLPath)
	if err != nil {
		t.Fatalf("Failed to execute GetOAuthLoginURL request: %v", err)
	}
	defer respURL.Body.Close()

	if respURL.StatusCode != http.StatusOK {
		t.Fatalf("GetOAuthLoginURL failed with status %d", respURL.StatusCode)
	}

	var urlResp struct {
		LoginUrl string `json:"loginUrl"`
	}
	_ = json.NewDecoder(respURL.Body).Decode(&urlResp)
	if urlResp.LoginUrl == "" {
		t.Fatal("Facebook login URL was empty")
	}

	parsedURL, err := url.Parse(urlResp.LoginUrl)
	if err != nil {
		t.Fatalf("Failed to parse Facebook login URL: %v", err)
	}
	stateVal := parsedURL.Query().Get("state")

	// 2. Perform LoginWithOAuth
	loginReq := map[string]interface{}{
		"provider":    "facebook",
		"code":        "facebook_auth_code_e2e",
		"redirectUri": redirectURI,
		"state":       stateVal,
	}
	reqBody, _ := json.Marshal(loginReq)

	respLogin, err := http.Post(baseURL+"/api/v1/auth/login/oauth", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to execute LoginWithOAuth request: %v", err)
	}
	defer respLogin.Body.Close()

	if respLogin.StatusCode != http.StatusOK {
		t.Fatalf("LoginWithOAuth failed with status %d", respLogin.StatusCode)
	}

	var loginResp struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		UserID       string `json:"userId"`
	}
	_ = json.NewDecoder(respLogin.Body).Decode(&loginResp)

	if loginResp.AccessToken == "" || loginResp.RefreshToken == "" || loginResp.UserID == "" {
		t.Fatalf("Login response returned empty values: %+v", loginResp)
	}

	// Verify DB state
	var user infraPostgres.UserModel
	if err := db.First(&user, "id = ?", loginResp.UserID).Error; err != nil {
		t.Fatalf("User record was not created: %v", err)
	}
	if user.Email != "facebook-e2e-user@example.com" {
		t.Errorf("Expected user email facebook-e2e-user@example.com, got %s", user.Email)
	}

	// 3. Perform RefreshToken
	refreshReq := map[string]interface{}{
		"refreshToken": loginResp.RefreshToken,
	}
	refreshBody, _ := json.Marshal(refreshReq)

	respRefresh, err := http.Post(baseURL+"/api/v1/auth/refresh", "application/json", bytes.NewBuffer(refreshBody))
	if err != nil {
		t.Fatalf("Failed to execute RefreshToken request: %v", err)
	}
	defer respRefresh.Body.Close()

	if respRefresh.StatusCode != http.StatusOK {
		t.Fatalf("RefreshToken failed with status %d", respRefresh.StatusCode)
	}

	var refreshResp struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(respRefresh.Body).Decode(&refreshResp)

	if refreshResp.AccessToken == "" || refreshResp.RefreshToken == "" {
		t.Fatalf("Refresh response returned empty values: %+v", refreshResp)
	}

	// 4. Perform Logout
	logoutReq := map[string]interface{}{
		"refreshToken": refreshResp.RefreshToken,
	}
	logoutBody, _ := json.Marshal(logoutReq)

	reqLogout, err := http.NewRequest("POST", baseURL+"/api/v1/auth/logout", bytes.NewBuffer(logoutBody))
	if err != nil {
		t.Fatalf("Failed to create Logout request: %v", err)
	}
	reqLogout.Header.Set("Content-Type", "application/json")
	reqLogout.Header.Set("X-User-Id", loginResp.UserID)
	reqLogout.Header.Set("Grpc-Metadata-X-User-Id", loginResp.UserID)

	respLogout, err := http.DefaultClient.Do(reqLogout)
	if err != nil {
		t.Fatalf("Failed to execute Logout request: %v", err)
	}
	defer respLogout.Body.Close()

	if respLogout.StatusCode != http.StatusOK {
		t.Fatalf("Logout failed with status %d", respLogout.StatusCode)
	}

	var logoutResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(respLogout.Body).Decode(&logoutResp)
	if !logoutResp.Success {
		t.Fatalf("Logout response success was false: %s", logoutResp.Message)
	}

	// Verify session has been deleted
	var sessionCount int64
	db.Model(&infraPostgres.SessionModel{}).Where("token = ?", hashToken(refreshResp.RefreshToken)).Count(&sessionCount)
	if sessionCount != 0 {
		t.Error("Expected session to be deleted from DB, but it still exists")
	}
}

func TestE2E_Logout_BOLA_Prevention(t *testing.T) {
	teardownMock := setupOAuthMock()
	defer teardownMock()

	baseURL, db, cleanup := startE2ETestServer(t)
	defer cleanup()

	// 1. Get OAuth Login URL for Google
	redirectURI := "http://localhost:3000/oauth/callback"
	loginURLPath := fmt.Sprintf("/api/v1/auth/oauth/login?provider=google&redirectUri=%s", url.QueryEscape(redirectURI))
	respURL, err := http.Get(baseURL + loginURLPath)
	if err != nil {
		t.Fatalf("Failed to execute GetOAuthLoginURL request: %v", err)
	}
	defer respURL.Body.Close()

	var urlResp struct {
		LoginUrl string `json:"loginUrl"`
	}
	_ = json.NewDecoder(respURL.Body).Decode(&urlResp)

	parsedURL, err := url.Parse(urlResp.LoginUrl)
	if err != nil {
		t.Fatalf("Failed to parse Google login URL: %v", err)
	}
	stateVal := parsedURL.Query().Get("state")

	// 2. Perform LoginWithOAuth
	loginReq := map[string]interface{}{
		"provider":    "google",
		"code":        "google_auth_code_e2e",
		"redirectUri": redirectURI,
		"state":       stateVal,
	}
	reqBody, _ := json.Marshal(loginReq)

	respLogin, err := http.Post(baseURL+"/api/v1/auth/login/oauth", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to execute LoginWithOAuth request: %v", err)
	}
	defer respLogin.Body.Close()

	var loginResp struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		UserID       string `json:"userId"`
	}
	_ = json.NewDecoder(respLogin.Body).Decode(&loginResp)

	// 3. Perform BOLA Logout Attempt (wrong UserID)
	logoutReq := map[string]interface{}{
		"refreshToken": loginResp.RefreshToken,
	}
	logoutBody, _ := json.Marshal(logoutReq)

	reqBOLA, err := http.NewRequest("POST", baseURL+"/api/v1/auth/logout", bytes.NewBuffer(logoutBody))
	if err != nil {
		t.Fatalf("Failed to create Logout request: %v", err)
	}
	reqBOLA.Header.Set("Content-Type", "application/json")
	reqBOLA.Header.Set("X-User-Id", "mismatched-user-uuid-12345")
	reqBOLA.Header.Set("Grpc-Metadata-X-User-Id", "mismatched-user-uuid-12345")

	respBOLA, err := http.DefaultClient.Do(reqBOLA)
	if err != nil {
		t.Fatalf("Failed to execute Logout request: %v", err)
	}
	defer respBOLA.Body.Close()

	if respBOLA.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got status %d", respBOLA.StatusCode)
	}

	var bolaResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(respBOLA.Body).Decode(&bolaResp)
	if bolaResp.Success {
		t.Fatal("BOLA logout succeeded but it should have failed due to user ID mismatch")
	}

	// Verify session still exists in DB
	var sessionCount int64
	db.Model(&infraPostgres.SessionModel{}).Where("token = ?", hashToken(loginResp.RefreshToken)).Count(&sessionCount)
	if sessionCount != 1 {
		t.Errorf("Expected 1 session to still exist, got %d", sessionCount)
	}
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
