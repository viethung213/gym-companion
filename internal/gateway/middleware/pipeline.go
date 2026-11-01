package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// PipelineOptions configures the security pipeline for Gateway.
type PipelineOptions struct {
	HostFilter     *HostFilter
	KeyProvider    KeyProvider
	InternalSecret string
	TokenTTL       time.Duration
	EnforceTLS     bool
}

// Pipeline orchestrates all Edge security layers for incoming gateway traffic.
type Pipeline struct {
	hostFilter  *HostFilter
	rateLimiter *RateLimiter
	jwtVal      *JWTValidator
	translator  *TokenTranslator
	enforceTLS  bool
}

// NewPipeline creates and wires the gateway security middleware pipeline.
func NewPipeline(opts PipelineOptions) *Pipeline {
	hf := opts.HostFilter
	if hf == nil {
		hf = NewHostFilter()
	}

	return &Pipeline{
		hostFilter:  hf,
		rateLimiter: NewRateLimiter(),
		jwtVal:      NewJWTValidator(opts.KeyProvider),
		translator:  NewTokenTranslator(opts.InternalSecret, opts.TokenTTL),
		enforceTLS:  opts.EnforceTLS,
	}
}

// Wrap chains all security checks around the core dispatcher handler.
func (p *Pipeline) Wrap(next http.Handler) http.Handler {
	// Base handler with JWT auth & Token translation
	authAndTranslate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		procedure := r.URL.Path
		if procedure == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		requiresAuth := IsAuthRequired(procedure)

		var userID, role string
		authHeader := r.Header.Get("Authorization")

		if requiresAuth {
			var err error
			userID, role, err = p.jwtVal.Verify(r.Context(), authHeader)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintf(w, `{"code":"UNAUTHENTICATED","message":"%s"}`, err.Error())
				return
			}
		} else {
			// Optional/Public auth
			if authHeader != "" {
				if uid, rRole, err := p.jwtVal.Verify(r.Context(), authHeader); err == nil {
					userID = uid
					role = rRole
				}
			}
			if userID == "" {
				userID = "guest"
				role = "anonymous"
			}
		}

		reqID := r.Header.Get(HeaderRequestID)
		internalToken, err := p.translator.Generate(userID, role, reqID)
		if err != nil {
			log.Printf("[Gateway Token Gen Error]: %v", err)
			http.Error(w, `{"code":"INTERNAL_ERROR","message":"Token generation failed"}`, http.StatusInternalServerError)
			return
		}

		// Attach Zero-Trust internal token and strip raw user bearer authorization
		r.Header.Set(HeaderInternalToken, internalToken)
		r.Header.Del("Authorization")

		next.ServeHTTP(w, r)
	})

	// Wrap with Rate Limiter
	rateLimited := p.rateLimiter.Wrap(authAndTranslate)

	// Wrap with Tracing
	traced := Tracing(rateLimited)

	// Wrap with Recovery
	recovered := Recovery(traced)

	// Wrap with Host/Authority Whitelist
	hostChecked := p.hostFilter.Wrap(recovered)

	// Outer-most: TLS Enforcement (if enabled)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.enforceTLS {
			isTLS := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
			if !isTLS {
				log.Printf("[Gateway Security] Rejected non-TLS request to %s from %s", r.URL.Path, r.RemoteAddr)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUpgradeRequired)
				_, _ = w.Write([]byte(`{"code":"TLS_REQUIRED","message":"HTTPS connection is required"}`))
				return
			}
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		hostChecked.ServeHTTP(w, r)
	})
}
