package gwerror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGWError_StandardResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		call       func(w http.ResponseWriter, r *http.Request)
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name: "NotFound default",
			call: func(w http.ResponseWriter, r *http.Request) {
				NotFound(w, r)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   CodeNotFound,
			wantMsg:    "Route not found",
		},
		{
			name: "Unauthenticated",
			call: func(w http.ResponseWriter, r *http.Request) {
				Unauthenticated(w, r, "missing or invalid token")
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeUnauthenticated,
			wantMsg:    "missing or invalid token",
		},
		{
			name: "Forbidden",
			call: func(w http.ResponseWriter, r *http.Request) {
				Forbidden(w, r, "access denied")
			},
			wantStatus: http.StatusForbidden,
			wantCode:   CodeForbidden,
			wantMsg:    "access denied",
		},
		{
			name: "CircuitOpen",
			call: func(w http.ResponseWriter, r *http.Request) {
				CircuitOpen(w, r, "all servers tripped")
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   CodeCircuitOpen,
			wantMsg:    "all servers tripped",
		},
		{
			name: "ServiceUnavailable",
			call: func(w http.ResponseWriter, r *http.Request) {
				ServiceUnavailable(w, r, "no balancer configured")
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   CodeServiceUnavailable,
			wantMsg:    "no balancer configured",
		},
		{
			name: "BadGateway",
			call: func(w http.ResponseWriter, r *http.Request) {
				BadGateway(w, r, "upstream failure")
			},
			wantStatus: http.StatusBadGateway,
			wantCode:   CodeBadGateway,
			wantMsg:    "upstream failure",
		},
		{
			name: "Internal",
			call: func(w http.ResponseWriter, r *http.Request) {
				Internal(w, r, "unexpected error")
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   CodeInternal,
			wantMsg:    "unexpected error",
		},
		{
			name: "RateLimited",
			call: func(w http.ResponseWriter, r *http.Request) {
				RateLimited(w, r, "too many requests")
			},
			wantStatus: http.StatusTooManyRequests,
			wantCode:   CodeRateLimited,
			wantMsg:    "too many requests",
		},
		{
			name: "TLSRequired",
			call: func(w http.ResponseWriter, r *http.Request) {
				TLSRequired(w, r)
			},
			wantStatus: http.StatusUpgradeRequired,
			wantCode:   CodeTLSRequired,
			wantMsg:    "HTTPS connection is required",
		},
		{
			name: "MethodNotAllowed",
			call: func(w http.ResponseWriter, r *http.Request) {
				MethodNotAllowed(w, r)
			},
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   CodeMethodNotAllowed,
			wantMsg:    "Method not allowed",
		},
		{
			name: "BadRequest",
			call: func(w http.ResponseWriter, r *http.Request) {
				BadRequest(w, r, "invalid JSON")
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeBadRequest,
			wantMsg:    "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Request-ID", "test-request-id-123")

			tt.call(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var resp Response
			err := json.NewDecoder(rec.Body).Decode(&resp)
			require.NoError(t, err)

			assert.Equal(t, tt.wantCode, resp.Code)
			assert.Equal(t, tt.wantMsg, resp.Message)
			assert.Equal(t, "test-request-id-123", resp.RequestID)
		})
	}
}
