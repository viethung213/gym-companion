package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTracing(t *testing.T) {
	t.Parallel()

	t.Run("generates new RequestID if not provided", func(t *testing.T) {
		var capturedReqID string
		handler := Tracing(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			capturedReqID = r.Header.Get(HeaderRequestID)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.NotEmpty(t, capturedReqID)
		assert.Equal(t, capturedReqID, rec.Header().Get(HeaderRequestID))
	})

	t.Run("preserves existing RequestID", func(t *testing.T) {
		existingID := "custom-uuid-12345"
		var capturedReqID string
		handler := Tracing(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			capturedReqID = r.Header.Get(HeaderRequestID)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(HeaderRequestID, existingID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, existingID, capturedReqID)
		assert.Equal(t, existingID, rec.Header().Get(HeaderRequestID))
	})
}
