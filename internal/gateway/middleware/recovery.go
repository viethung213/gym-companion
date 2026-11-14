package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/viethung213/gym-companion/internal/gateway/gwerror"
)

// Recovery recovers from panics and logs the stack trace.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(r.Context(), "Gateway panic recovered",
					"method", r.Method,
					"path", r.URL.Path,
					"panic", fmt.Sprint(rec),
				)
				gwerror.Internal(w, r, "Gateway internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
