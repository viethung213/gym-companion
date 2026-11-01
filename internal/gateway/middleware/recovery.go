package middleware

import (
	"log"
	"net/http"
)

// Recovery recovers from panics and logs the stack trace.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[Gateway Panic Recovered] %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, `{"code":"INTERNAL_ERROR","message":"Gateway internal error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
