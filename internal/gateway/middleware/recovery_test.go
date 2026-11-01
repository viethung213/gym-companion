package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecovery(t *testing.T) {
	t.Parallel()

	panickingHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("unexpected runtime panic!")
	})

	handler := Recovery(panickingHandler)

	req := httptest.NewRequest(http.MethodGet, "/test-panic", nil)
	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		handler.ServeHTTP(rec, req)
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "Gateway internal error")
}
