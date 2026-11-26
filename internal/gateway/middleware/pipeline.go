package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/gateway/gwerror"
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
		rateLimiter: NewRateLimiter().WithInternalSecret(opts.InternalSecret),
		jwtVal:      NewJWTValidator(opts.KeyProvider),
		translator:  NewTokenTranslator(opts.InternalSecret, opts.TokenTTL),
		enforceTLS:  opts.EnforceTLS,
	}
}

// Wrap chains all security checks around the core dispatcher handler.
func (p *Pipeline) Wrap(next http.Handler) http.Handler {
	// Base handler with JWT auth & Token translation
	authAndTranslate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Perimeter check (defense-in-depth): external clients must never supply X-Internal-Token
		if r.Header.Get(HeaderInternalToken) != "" {
			r.Header.Del(HeaderInternalToken)
			gwerror.Forbidden(w, r, "external X-Internal-Token is forbidden")
			return
		}

		procedure := r.URL.Path
		if procedure == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		requiresAuth := IsAuthRequired(procedure)

		var userID, role string
		authHeader := r.Header.Get("Authorization")

		switch {
		case requiresAuth:
			var err error
			userID, role, err = p.jwtVal.Verify(r.Context(), authHeader)
			if err != nil {
				gwerror.Unauthenticated(w, r, err.Error())
				return
			}
		default:
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
			slog.ErrorContext(r.Context(), "Gateway token generation failed", "error", err)
			gwerror.Internal(w, r, "Token generation failed")
			return
		}

		// Attach Zero-Trust internal token, Gateway user headers, and strip raw user bearer authorization
		r.Header.Set(HeaderInternalToken, internalToken)
		r.Header.Set("X-User-Id", userID)
		r.Header.Set("X-User-Role", role)
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

	// Outer-most: Perimeter Sanitization & TLS Enforcement (if enabled)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Perimeter Security: Gateway at the edge must reject and strip any incoming X-Internal-Token from external callers
		if r.Header.Get(HeaderInternalToken) != "" {
			r.Header.Del(HeaderInternalToken)
			gwerror.Forbidden(w, r, "external X-Internal-Token is forbidden at perimeter")
			return
		}

		if p.enforceTLS {
			isTLS := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
			if !isTLS {
				slog.WarnContext(r.Context(), "Rejected non-TLS request",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				gwerror.TLSRequired(w, r)
				return
			}
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		hostChecked.ServeHTTP(w, r)
	})
}

// WrapInternal chains security and verification checks for internal service-to-service ingress (Approach 2).
// It strictly validates X-Internal-Token and blocks unauthorized callers, while applying rate limiting,
// distributed tracing, and recovery to prevent runaway module spam loops.
func (p *Pipeline) WrapInternal(next http.Handler) http.Handler {
	internalAuth := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		procedure := r.URL.Path
		if procedure == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		tokenStr := r.Header.Get(HeaderInternalToken)
		if tokenStr == "" {
			gwerror.Unauthenticated(w, r, "missing X-Internal-Token for internal gateway call")
			return
		}

		claims, err := p.translator.Validate(tokenStr)
		if err != nil {
			gwerror.Unauthenticated(w, r, fmt.Sprintf("invalid internal token: %v", err))
			return
		}

		// Inject verified user and role headers into the request for downstream routing
		if claims.UserID != "" {
			r.Header.Set("X-User-Id", claims.UserID)
		}
		if claims.Role != "" {
			r.Header.Set("X-User-Role", claims.Role)
		}

		// Preserve or propagate request tracing ID from claims
		if claims.RequestID != "" {
			r.Header.Set(HeaderRequestID, claims.RequestID)
		}

		next.ServeHTTP(w, r)
	})

	// Wrap with Rate Limiter (protects against internal module spamming/looping)
	rateLimited := p.rateLimiter.Wrap(internalAuth)

	// Wrap with Tracing (maintains distributed trace ID)
	traced := Tracing(rateLimited)

	// Wrap with Recovery
	return Recovery(traced)
}
