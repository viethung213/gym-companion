//go:build e2e

package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	infraPostgres "github.com/viethung213/gym-companion/internal/auth/infrastructure/persistence/postgres"
)

const (
	authServicePrefix = "/contracts.generic.auth.v1.service.AuthService/"
)

// TestE2E_JWKS_And_KeyRotation tests public keys retrieval and manual rotation flows.
func TestE2E_JWKS_And_KeyRotation(t *testing.T) {
	baseURL, _, cleanup := startE2ETestServer(t)
	defer cleanup()

	// 1. Fetch JWKS Initially
	resp, err := http.Post(baseURL+authServicePrefix+"GetJWKS", "application/json", bytes.NewBuffer([]byte("{}")))
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
	respRotate, err := http.Post(baseURL+authServicePrefix+"RotateKeys", "application/json", bytes.NewBuffer([]byte("{}")))
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
	respNew, err := http.Post(baseURL+authServicePrefix+"GetJWKS", "application/json", bytes.NewBuffer([]byte("{}")))
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
	urlReq := map[string]interface{}{
		"provider":    "google",
		"redirectUri": redirectURI,
	}
	urlReqBody, _ := json.Marshal(urlReq)

	respURL, err := http.Post(baseURL+authServicePrefix+"GetOAuthLoginURL", "application/json", bytes.NewBuffer(urlReqBody))
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

	// State parsing from URL
	stateVal := "test-oauth-state-google"
	if stateIdx := bytes.Index([]byte(urlResp.LoginUrl), []byte("state=")); stateIdx != -1 {
		stateVal = string([]byte(urlResp.LoginUrl)[stateIdx+6:])
		if ampIdx := bytes.Index([]byte(stateVal), []byte("&")); ampIdx != -1 {
			stateVal = stateVal[:ampIdx]
		}
	}

	// 2. Perform LoginWithOAuth
	loginReq := map[string]interface{}{
		"provider":    "google",
		"code":        "google_auth_code_e2e",
		"redirectUri": redirectURI,
		"state":       stateVal,
	}
	reqBody, _ := json.Marshal(loginReq)

	respLogin, err := http.Post(baseURL+authServicePrefix+"LoginWithOAuth", "application/json", bytes.NewBuffer(reqBody))
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
	var identity infraPostgres.UserIdentityModel
	if err := db.First(&identity, "user_id = ? AND identity_type = 'google'", loginResp.UserID).Error; err != nil {
		t.Fatalf("User identity record was not created: %v", err)
	}
	if identity.Identifier != "11223344556677889900" {
		t.Errorf("Expected google identifier 11223344556677889900, got %s", identity.Identifier)
	}

	// 3. Perform RefreshToken
	refreshReq := map[string]interface{}{
		"refreshToken": loginResp.RefreshToken,
	}
	refreshBody, _ := json.Marshal(refreshReq)

	respRefresh, err := http.Post(baseURL+authServicePrefix+"RefreshToken", "application/json", bytes.NewBuffer(refreshBody))
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

	reqLogout, err := http.NewRequest("POST", baseURL+authServicePrefix+"Logout", bytes.NewBuffer(logoutBody))
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
	urlReq := map[string]interface{}{
		"provider":    "facebook",
		"redirectUri": redirectURI,
	}
	urlReqBody, _ := json.Marshal(urlReq)

	respURL, err := http.Post(baseURL+authServicePrefix+"GetOAuthLoginURL", "application/json", bytes.NewBuffer(urlReqBody))
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

	// State parsing
	stateVal := "test-oauth-state-facebook"
	if stateIdx := bytes.Index([]byte(urlResp.LoginUrl), []byte("state=")); stateIdx != -1 {
		stateVal = string([]byte(urlResp.LoginUrl)[stateIdx+6:])
		if ampIdx := bytes.Index([]byte(stateVal), []byte("&")); ampIdx != -1 {
			stateVal = stateVal[:ampIdx]
		}
	}

	// 2. Perform LoginWithOAuth
	loginReq := map[string]interface{}{
		"provider":    "facebook",
		"code":        "facebook_auth_code_e2e",
		"redirectUri": redirectURI,
		"state":       stateVal,
	}
	reqBody, _ := json.Marshal(loginReq)

	respLogin, err := http.Post(baseURL+authServicePrefix+"LoginWithOAuth", "application/json", bytes.NewBuffer(reqBody))
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
	var identity infraPostgres.UserIdentityModel
	if err := db.First(&identity, "user_id = ? AND identity_type = 'facebook'", loginResp.UserID).Error; err != nil {
		t.Fatalf("User identity record was not created: %v", err)
	}
	if identity.Identifier != "22334455667788990011" {
		t.Errorf("Expected facebook identifier 22334455667788990011, got %s", identity.Identifier)
	}

	// 3. Perform RefreshToken
	refreshReq := map[string]interface{}{
		"refreshToken": loginResp.RefreshToken,
	}
	refreshBody, _ := json.Marshal(refreshReq)

	respRefresh, err := http.Post(baseURL+authServicePrefix+"RefreshToken", "application/json", bytes.NewBuffer(refreshBody))
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

	reqLogout, err := http.NewRequest("POST", baseURL+authServicePrefix+"Logout", bytes.NewBuffer(logoutBody))
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
	urlReq := map[string]interface{}{
		"provider":    "google",
		"redirectUri": redirectURI,
	}
	urlReqBody, _ := json.Marshal(urlReq)

	respURL, err := http.Post(baseURL+authServicePrefix+"GetOAuthLoginURL", "application/json", bytes.NewBuffer(urlReqBody))
	if err != nil {
		t.Fatalf("Failed to execute GetOAuthLoginURL request: %v", err)
	}
	defer respURL.Body.Close()

	var urlResp struct {
		LoginUrl string `json:"loginUrl"`
	}
	_ = json.NewDecoder(respURL.Body).Decode(&urlResp)

	stateVal := "test-oauth-state-bola"
	if stateIdx := bytes.Index([]byte(urlResp.LoginUrl), []byte("state=")); stateIdx != -1 {
		stateVal = string([]byte(urlResp.LoginUrl)[stateIdx+6:])
		if ampIdx := bytes.Index([]byte(stateVal), []byte("&")); ampIdx != -1 {
			stateVal = stateVal[:ampIdx]
		}
	}

	// 2. Perform LoginWithOAuth
	loginReq := map[string]interface{}{
		"provider":    "google",
		"code":        "google_auth_code_e2e",
		"redirectUri": redirectURI,
		"state":       stateVal,
	}
	reqBody, _ := json.Marshal(loginReq)

	respLogin, err := http.Post(baseURL+authServicePrefix+"LoginWithOAuth", "application/json", bytes.NewBuffer(reqBody))
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

	reqBOLA, err := http.NewRequest("POST", baseURL+authServicePrefix+"Logout", bytes.NewBuffer(logoutBody))
	if err != nil {
		t.Fatalf("Failed to create Logout request: %v", err)
	}
	reqBOLA.Header.Set("Content-Type", "application/json")
	reqBOLA.Header.Set("X-User-Id", "mismatched-user-uuid-12345")

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

// TestE2E_Register_And_CredentialsLogin verifies registration and password login with Email and Phone.
func TestE2E_Register_And_CredentialsLogin(t *testing.T) {
	baseURL, db, cleanup := startE2ETestServer(t)
	defer cleanup()

	// 1. Register with Email
	regEmailReq := map[string]interface{}{
		"identifier":  "newbie@example.com",
		"password":    "StrongPassword123!",
		"fullName":    "Newbie User",
		"gender":      "MALE",
		"dateOfBirth": "1995-10-20",
	}
	regEmailBody, _ := json.Marshal(regEmailReq)

	respRegEmail, err := http.Post(baseURL+authServicePrefix+"Register", "application/json", bytes.NewBuffer(regEmailBody))
	if err != nil {
		t.Fatalf("Failed to execute Register request: %v", err)
	}
	defer respRegEmail.Body.Close()

	if respRegEmail.StatusCode != http.StatusOK {
		t.Fatalf("Register with email failed with status %d", respRegEmail.StatusCode)
	}

	// Verify DB state for registered email user
	var identityEmail infraPostgres.UserIdentityModel
	if err := db.First(&identityEmail, "identifier = ?", "newbie@example.com").Error; err != nil {
		t.Fatalf("User identity was not saved to DB: %v", err)
	}
	if identityEmail.IdentityType != "email" {
		t.Errorf("Expected identity_type email, got %s", identityEmail.IdentityType)
	}

	// 2. Register Duplicate Email (Expect Conflict / Non-200)
	respDup, err := http.Post(baseURL+authServicePrefix+"Register", "application/json", bytes.NewBuffer(regEmailBody))
	if err != nil {
		t.Fatalf("Failed to execute duplicate Register request: %v", err)
	}
	defer respDup.Body.Close()
	if respDup.StatusCode == http.StatusOK {
		t.Fatalf("Expected duplicate registration to fail, but got HTTP 200")
	}

	// 3. Login with Email and Correct Password
	loginEmailReq := map[string]interface{}{
		"identifier": "newbie@example.com",
		"password":   "StrongPassword123!",
	}
	loginEmailBody, _ := json.Marshal(loginEmailReq)

	respLoginEmail, err := http.Post(baseURL+authServicePrefix+"Login", "application/json", bytes.NewBuffer(loginEmailBody))
	if err != nil {
		t.Fatalf("Failed to execute Login request: %v", err)
	}
	defer respLoginEmail.Body.Close()

	if respLoginEmail.StatusCode != http.StatusOK {
		t.Fatalf("Login with email failed with status %d", respLoginEmail.StatusCode)
	}

	var loginResp struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		UserId       string `json:"userId"`
	}
	_ = json.NewDecoder(respLoginEmail.Body).Decode(&loginResp)
	if loginResp.AccessToken == "" || loginResp.RefreshToken == "" || loginResp.UserId == "" {
		t.Fatalf("Login response missing token or user id: %+v", loginResp)
	}

	// 4. Login with Wrong Password (Expect 401)
	loginWrongReq := map[string]interface{}{
		"identifier": "newbie@example.com",
		"password":   "WrongPassword!",
	}
	loginWrongBody, _ := json.Marshal(loginWrongReq)

	respLoginWrong, err := http.Post(baseURL+authServicePrefix+"Login", "application/json", bytes.NewBuffer(loginWrongBody))
	if err != nil {
		t.Fatalf("Failed to execute Login request: %v", err)
	}
	defer respLoginWrong.Body.Close()
	if respLoginWrong.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected HTTP 401 for wrong password, got status %d", respLoginWrong.StatusCode)
	}

	// 5. Register with Phone (Local domestic format: 0912345678 -> normalized to +84912345678)
	regPhoneReq := map[string]interface{}{
		"identifier":  "0912345678",
		"password":    "StrongPassword123!",
		"fullName":    "Phone User",
		"gender":      "FEMALE",
		"dateOfBirth": "1998-05-15",
	}
	regPhoneBody, _ := json.Marshal(regPhoneReq)

	respRegPhone, err := http.Post(baseURL+authServicePrefix+"Register", "application/json", bytes.NewBuffer(regPhoneBody))
	if err != nil {
		t.Fatalf("Failed to execute Register with phone request: %v", err)
	}
	defer respRegPhone.Body.Close()

	if respRegPhone.StatusCode != http.StatusOK {
		t.Fatalf("Register with phone failed with status %d", respRegPhone.StatusCode)
	}

	// Verify DB state for normalized phone
	var identityPhone infraPostgres.UserIdentityModel
	if err := db.First(&identityPhone, "identifier = ?", "+84912345678").Error; err != nil {
		t.Fatalf("Phone user identity was not saved with normalized format +84912345678: %v", err)
	}
	if identityPhone.IdentityType != "phone" {
		t.Errorf("Expected identity_type phone, got %s", identityPhone.IdentityType)
	}

	// 6. Login with Phone and Password
	loginPhoneReq := map[string]interface{}{
		"identifier": "0912345678",
		"password":   "StrongPassword123!",
	}
	loginPhoneBody, _ := json.Marshal(loginPhoneReq)

	respLoginPhone, err := http.Post(baseURL+authServicePrefix+"Login", "application/json", bytes.NewBuffer(loginPhoneBody))
	if err != nil {
		t.Fatalf("Failed to execute Login with phone request: %v", err)
	}
	defer respLoginPhone.Body.Close()

	if respLoginPhone.StatusCode != http.StatusOK {
		t.Fatalf("Login with phone failed with status %d", respLoginPhone.StatusCode)
	}
}

// TestE2E_ChangePassword verifies changing password while authenticated.
func TestE2E_ChangePassword(t *testing.T) {
	baseURL, _, cleanup := startE2ETestServer(t)
	defer cleanup()

	// 1. Register user
	regReq := map[string]interface{}{
		"identifier":  "changepwd@example.com",
		"password":    "InitialPass123!",
		"fullName":    "Change Password User",
		"gender":      "MALE",
		"dateOfBirth": "1992-02-02",
	}
	regBody, _ := json.Marshal(regReq)
	respReg, err := http.Post(baseURL+authServicePrefix+"Register", "application/json", bytes.NewBuffer(regBody))
	if err != nil || respReg.StatusCode != http.StatusOK {
		t.Fatalf("Failed to register test user: %v", err)
	}
	respReg.Body.Close()

	// 2. Login to get user ID
	loginReq := map[string]interface{}{
		"identifier": "changepwd@example.com",
		"password":   "InitialPass123!",
	}
	loginBody, _ := json.Marshal(loginReq)
	respLogin, err := http.Post(baseURL+authServicePrefix+"Login", "application/json", bytes.NewBuffer(loginBody))
	if err != nil || respLogin.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login test user: %v", err)
	}
	var loginResp struct {
		UserId string `json:"userId"`
	}
	_ = json.NewDecoder(respLogin.Body).Decode(&loginResp)
	respLogin.Body.Close()

	// 3. Change password with wrong old password (Expect 401)
	wrongOldPwdReq := map[string]interface{}{
		"oldPassword":     "WrongOldPass!",
		"newPassword":     "BrandNewPass123!",
		"confirmPassword": "BrandNewPass123!",
	}
	wrongOldPwdBody, _ := json.Marshal(wrongOldPwdReq)
	reqWrong, _ := http.NewRequest("POST", baseURL+authServicePrefix+"ChangePassword", bytes.NewBuffer(wrongOldPwdBody))
	reqWrong.Header.Set("Content-Type", "application/json")
	reqWrong.Header.Set("X-User-Id", loginResp.UserId)

	respWrong, err := http.DefaultClient.Do(reqWrong)
	if err != nil {
		t.Fatalf("Failed to call ChangePassword: %v", err)
	}
	defer respWrong.Body.Close()
	if respWrong.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected HTTP 401 for wrong old password, got status %d", respWrong.StatusCode)
	}

	// 4. Change password with valid old password
	validChangeReq := map[string]interface{}{
		"oldPassword":     "InitialPass123!",
		"newPassword":     "BrandNewPass123!",
		"confirmPassword": "BrandNewPass123!",
	}
	validChangeBody, _ := json.Marshal(validChangeReq)
	reqValid, _ := http.NewRequest("POST", baseURL+authServicePrefix+"ChangePassword", bytes.NewBuffer(validChangeBody))
	reqValid.Header.Set("Content-Type", "application/json")
	reqValid.Header.Set("X-User-Id", loginResp.UserId)

	respValid, err := http.DefaultClient.Do(reqValid)
	if err != nil {
		t.Fatalf("Failed to call ChangePassword: %v", err)
	}
	defer respValid.Body.Close()
	if respValid.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200 for valid ChangePassword, got status %d", respValid.StatusCode)
	}

	// 5. Verify old password no longer works
	respOldLogin, err := http.Post(baseURL+authServicePrefix+"Login", "application/json", bytes.NewBuffer(loginBody))
	if err != nil {
		t.Fatalf("Failed to test old password login: %v", err)
	}
	defer respOldLogin.Body.Close()
	if respOldLogin.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Old password should no longer work, but got status %d", respOldLogin.StatusCode)
	}

	// 6. Verify new password works
	newLoginReq := map[string]interface{}{
		"identifier": "changepwd@example.com",
		"password":   "BrandNewPass123!",
	}
	newLoginBody, _ := json.Marshal(newLoginReq)
	respNewLogin, err := http.Post(baseURL+authServicePrefix+"Login", "application/json", bytes.NewBuffer(newLoginBody))
	if err != nil {
		t.Fatalf("Failed to test new password login: %v", err)
	}
	defer respNewLogin.Body.Close()
	if respNewLogin.StatusCode != http.StatusOK {
		t.Fatalf("New password login failed with status %d", respNewLogin.StatusCode)
	}
}

// TestE2E_OTP_And_ResetPassword verifies SendOTP, VerifyOTP, and ResetPassword flows.
func TestE2E_OTP_And_ResetPassword(t *testing.T) {
	baseURL, db, cleanup := startE2ETestServer(t)
	defer cleanup()

	// 1. Register target user
	regReq := map[string]interface{}{
		"identifier":  "forgotuser@example.com",
		"password":    "InitialPass123!",
		"fullName":    "Forgot User",
		"gender":      "MALE",
		"dateOfBirth": "1994-04-04",
	}
	regBody, _ := json.Marshal(regReq)
	respReg, err := http.Post(baseURL+authServicePrefix+"Register", "application/json", bytes.NewBuffer(regBody))
	if err != nil || respReg.StatusCode != http.StatusOK {
		t.Fatalf("Failed to register target user: %v", err)
	}
	respReg.Body.Close()

	// 2. Request OTP for Password Reset
	sendOTPReq := map[string]interface{}{
		"identifier": "forgotuser@example.com",
		"purpose":    "reset_password",
	}
	sendOTPBody, _ := json.Marshal(sendOTPReq)
	respSendOTP, err := http.Post(baseURL+authServicePrefix+"SendOTP", "application/json", bytes.NewBuffer(sendOTPBody))
	if err != nil {
		t.Fatalf("Failed to call SendOTP: %v", err)
	}
	defer respSendOTP.Body.Close()

	if respSendOTP.StatusCode != http.StatusOK {
		t.Fatalf("SendOTP failed with status %d", respSendOTP.StatusCode)
	}

	var sendOTPResp struct {
		Success  bool   `json:"success"`
		OtpToken string `json:"otpToken"`
	}
	_ = json.NewDecoder(respSendOTP.Body).Decode(&sendOTPResp)
	if !sendOTPResp.Success || sendOTPResp.OtpToken == "" {
		t.Fatalf("SendOTP returned unsuccessful response: %+v", sendOTPResp)
	}

	// 3. Retrieve the generated plain code from outbox record payload
	var outboxRecord infraPostgres.OutboxModel
	if err := db.Where("event_type ILIKE ?", "%otpsent%").Order("created_at DESC").First(&outboxRecord).Error; err != nil {
		t.Fatalf("Failed to find OTPSent event in outbox: %v", err)
	}

	var envelope struct {
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(outboxRecord.Payload, &envelope); err != nil {
		t.Fatalf("Failed to unmarshal outbox payload: %v", err)
	}
	plainOTPCode := envelope.Data.Code
	if plainOTPCode == "" {
		t.Fatalf("OTP code in outbox payload was empty")
	}

	// 4. Verify OTP with Wrong Code (Expect non-200)
	wrongVerifyReq := map[string]interface{}{
		"otpToken": sendOTPResp.OtpToken,
		"code":     "000000",
	}
	wrongVerifyBody, _ := json.Marshal(wrongVerifyReq)
	respWrongVerify, err := http.Post(baseURL+authServicePrefix+"VerifyOTP", "application/json", bytes.NewBuffer(wrongVerifyBody))
	if err != nil {
		t.Fatalf("Failed to call VerifyOTP with wrong code: %v", err)
	}
	defer respWrongVerify.Body.Close()
	if respWrongVerify.StatusCode == http.StatusOK {
		t.Fatalf("Expected VerifyOTP with wrong code to fail, got HTTP 200")
	}

	// 5. Verify OTP with Correct Code
	validVerifyReq := map[string]interface{}{
		"otpToken": sendOTPResp.OtpToken,
		"code":     plainOTPCode,
	}
	validVerifyBody, _ := json.Marshal(validVerifyReq)
	respValidVerify, err := http.Post(baseURL+authServicePrefix+"VerifyOTP", "application/json", bytes.NewBuffer(validVerifyBody))
	if err != nil {
		t.Fatalf("Failed to call VerifyOTP: %v", err)
	}
	defer respValidVerify.Body.Close()

	if respValidVerify.StatusCode != http.StatusOK {
		t.Fatalf("VerifyOTP failed with status %d", respValidVerify.StatusCode)
	}

	var verifyResp struct {
		Valid      bool   `json:"valid"`
		ResetToken string `json:"resetToken"`
	}
	_ = json.NewDecoder(respValidVerify.Body).Decode(&verifyResp)
	if !verifyResp.Valid || verifyResp.ResetToken == "" {
		t.Fatalf("VerifyOTP did not return valid resetToken: %+v", verifyResp)
	}

	// 6. Reset Password with reset_token
	resetReq := map[string]interface{}{
		"resetToken":      verifyResp.ResetToken,
		"newPassword":     "BrandNewResetPass123!",
		"confirmPassword": "BrandNewResetPass123!",
	}
	resetBody, _ := json.Marshal(resetReq)
	respReset, err := http.Post(baseURL+authServicePrefix+"ResetPassword", "application/json", bytes.NewBuffer(resetBody))
	if err != nil {
		t.Fatalf("Failed to call ResetPassword: %v", err)
	}
	defer respReset.Body.Close()

	if respReset.StatusCode != http.StatusOK {
		t.Fatalf("ResetPassword failed with status %d", respReset.StatusCode)
	}

	// 7. Login with newly reset password
	newLoginReq := map[string]interface{}{
		"identifier": "forgotuser@example.com",
		"password":   "BrandNewResetPass123!",
	}
	newLoginBody, _ := json.Marshal(newLoginReq)
	respNewLogin, err := http.Post(baseURL+authServicePrefix+"Login", "application/json", bytes.NewBuffer(newLoginBody))
	if err != nil {
		t.Fatalf("Failed to login after reset password: %v", err)
	}
	defer respNewLogin.Body.Close()

	if respNewLogin.StatusCode != http.StatusOK {
		t.Fatalf("Login with reset password failed with status %d", respNewLogin.StatusCode)
	}
}
