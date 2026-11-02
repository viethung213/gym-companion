package gwerror

import (
	"encoding/json"
	"net/http"
)

// Response defines the standardized JSON error structure returned by the API Gateway.
type Response struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

// Gateway standardized error codes compatible with ConnectRPC and gRPC status codes.
const (
	CodeNotFound           = "NOT_FOUND"
	CodeUnauthenticated    = "UNAUTHENTICATED"
	CodeForbidden          = "FORBIDDEN"
	CodeRateLimited        = "RESOURCE_EXHAUSTED"
	CodeCircuitOpen        = "CIRCUIT_OPEN"
	CodeServiceUnavailable = "UNAVAILABLE"
	CodeBadGateway         = "BAD_GATEWAY"
	CodeInternal           = "INTERNAL_ERROR"
	CodeTLSRequired        = "TLS_REQUIRED"
	CodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	CodeBadRequest         = "BAD_REQUEST"
)

// Write writes a standardized JSON error response with the current request's X-Request-ID.
func Write(w http.ResponseWriter, r *http.Request, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	var reqID string
	if r != nil {
		reqID = r.Header.Get("X-Request-ID")
	}

	_ = json.NewEncoder(w).Encode(Response{
		Code:      code,
		Message:   message,
		RequestID: reqID,
	})
}

// NotFound writes a 404 NOT_FOUND error response.
func NotFound(w http.ResponseWriter, r *http.Request, message ...string) {
	msg := "Route not found"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	Write(w, r, http.StatusNotFound, CodeNotFound, msg)
}

// Unauthenticated writes a 401 UNAUTHENTICATED error response.
func Unauthenticated(w http.ResponseWriter, r *http.Request, message string) {
	Write(w, r, http.StatusUnauthorized, CodeUnauthenticated, message)
}

// Forbidden writes a 403 FORBIDDEN error response.
func Forbidden(w http.ResponseWriter, r *http.Request, message string) {
	Write(w, r, http.StatusForbidden, CodeForbidden, message)
}

// ServiceUnavailable writes a 503 UNAVAILABLE error response.
func ServiceUnavailable(w http.ResponseWriter, r *http.Request, message string) {
	Write(w, r, http.StatusServiceUnavailable, CodeServiceUnavailable, message)
}

// CircuitOpen writes a 503 CIRCUIT_OPEN error response indicating all upstream instances are tripped.
func CircuitOpen(w http.ResponseWriter, r *http.Request, message string) {
	Write(w, r, http.StatusServiceUnavailable, CodeCircuitOpen, message)
}

// BadGateway writes a 502 BAD_GATEWAY error response.
func BadGateway(w http.ResponseWriter, r *http.Request, message string) {
	Write(w, r, http.StatusBadGateway, CodeBadGateway, message)
}

// Internal writes a 500 INTERNAL_ERROR error response.
func Internal(w http.ResponseWriter, r *http.Request, message string) {
	Write(w, r, http.StatusInternalServerError, CodeInternal, message)
}

// RateLimited writes a 429 RESOURCE_EXHAUSTED error response.
func RateLimited(w http.ResponseWriter, r *http.Request, message string) {
	Write(w, r, http.StatusTooManyRequests, CodeRateLimited, message)
}

// TLSRequired writes a 426 UPGRADE_REQUIRED error response.
func TLSRequired(w http.ResponseWriter, r *http.Request) {
	Write(w, r, http.StatusUpgradeRequired, CodeTLSRequired, "HTTPS connection is required")
}

// MethodNotAllowed writes a 405 METHOD_NOT_ALLOWED error response.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	Write(w, r, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "Method not allowed")
}

// BadRequest writes a 400 BAD_REQUEST error response.
func BadRequest(w http.ResponseWriter, r *http.Request, message string) {
	Write(w, r, http.StatusBadRequest, CodeBadRequest, message)
}
