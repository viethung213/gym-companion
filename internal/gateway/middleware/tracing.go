package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

// Tracing ensures every request has an X-Request-ID for distributed tracing.
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderRequestID)
		if reqID == "" {
			reqID = uuid.New().String()
		}
		w.Header().Set(HeaderRequestID, reqID)
		r.Header.Set(HeaderRequestID, reqID)

		next.ServeHTTP(w, r)
	})
}
