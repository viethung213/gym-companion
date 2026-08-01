package middleware

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// KeyProvider defines the interface to fetch the public key PEM for JWT verification.
type KeyProvider interface {
	GetPublicKeyPEM(ctx context.Context, kid string) (string, error)
}

// ContextKey represents a context key type to avoid collisions.
type ContextKey string

const (
	// UserIDKey is the context key for the user ID.
	UserIDKey ContextKey = "userId"
	// UserRoleKey is the context key for the user role.
	UserRoleKey ContextKey = "userRole"
)

var (
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	errMissingMetadata   = errors.New("missing metadata")
	errMissingAuthHeader = errors.New("missing authorization header")
	errInvalidAuthFormat = errors.New("invalid authorization format (expected Bearer <token>)")
)

type rsaKeyCache struct {
	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
	kp   KeyProvider
}

func newRSAKeyCache(kp KeyProvider) *rsaKeyCache {
	return &rsaKeyCache{
		keys: make(map[string]*rsa.PublicKey),
		kp:   kp,
	}
}

func (c *rsaKeyCache) GetPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if c.kp == nil {
		return nil, errors.New("key provider not initialized")
	}

	c.mu.RLock()
	pubKey, exists := c.keys[kid]
	c.mu.RUnlock()
	if exists {
		return pubKey, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if pubKey, exists = c.keys[kid]; exists {
		return pubKey, nil
	}

	pubKeyPEM, err := c.kp.GetPublicKeyPEM(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("get public key: %w", err)
	}

	pubKey, err = jwt.ParseRSAPublicKeyFromPEM([]byte(pubKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse rsa public key: %w", err)
	}

	c.keys[kid] = pubKey
	return pubKey, nil
}

type Actor struct {
	UserID string
	Role   string
}

func (a Actor) IsAdmin() bool {
	return strings.EqualFold(a.Role, "Admin")
}

func RequireAuthenticated(ctx context.Context) (Actor, error) {
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok || userID == "" {
		return Actor{}, ErrUnauthorized
	}
	role, _ := ctx.Value(UserRoleKey).(string)
	return Actor{
		UserID: userID,
		Role:   role,
	}, nil
}

func RequireAdmin(ctx context.Context) (Actor, error) {
	actor, err := RequireAuthenticated(ctx)
	if err != nil {
		return Actor{}, err
	}
	if !actor.IsAdmin() {
		return Actor{}, ErrForbidden
	}
	return actor, nil
}

// UnaryAuthInterceptor intercepts unary gRPC requests to validate JWT tokens.
func UnaryAuthInterceptor(kp KeyProvider) grpc.UnaryServerInterceptor {
	keyCache := newRSAKeyCache(kp)

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 1. Check if method bypasses auth based on proto contract annotations
		required := isAuthRequired(info.FullMethod)

		// 2. Extract token from metadata
		tokenStr, err := extractToken(ctx)
		if err != nil {
			// If auth header is completely missing on a public/optional route, proceed as anonymous
			if errors.Is(err, errMissingMetadata) || errors.Is(err, errMissingAuthHeader) {
				if required {
					return nil, status.Errorf(codes.Unauthenticated, "authentication required: %v", err)
				}
				return handler(ctx, req)
			}
			// Malformed authorization header (e.g. invalid format) -> always reject
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization header format: %v", err)
		}

		// 3. Validate token: If token was provided, it MUST be valid regardless of route auth requirement
		userID, role, err := parseAndValidateToken(ctx, tokenStr, keyCache)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		// 4. Inject identity into context
		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, UserRoleKey, role)

		return handler(ctx, req)
	}
}

func parseAndValidateToken(
	ctx context.Context,
	tokenStr string,
	cache *rsaKeyCache,
) (userID, role string, err error) {
	if cache == nil {
		return "", "", errors.New("key cache not initialized")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, errors.New("missing kid in token header")
		}

		return cache.GetPublicKey(ctx, kid)
	})

	if err != nil {
		return "", "", err
	}

	if !token.Valid {
		return "", "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid claims")
	}

	userID, ok = claims["sub"].(string)
	if !ok || userID == "" {
		return "", "", errors.New("missing sub claim")
	}
	role, ok = claims["role"].(string)
	if !ok || role == "" {
		return "", "", errors.New("missing role claim")
	}

	return userID, role, nil
}

// isAuthRequired checks the compiled protobuf annotations to see
// if the method requires BearerAuth.
func isAuthRequired(fullMethod string) bool {
	// Public endpoints that do not require JWT authentication
	if strings.HasSuffix(fullMethod, "/GetMotionSpecification") ||
		strings.HasSuffix(fullMethod, "/GetCatalogMetadata") ||
		strings.HasSuffix(fullMethod, "/SearchExercises") ||
		strings.HasSuffix(fullMethod, "/GetExercise") {
		return false
	}

	// FullMethod format: "/package.Service/Method"

	parts := strings.Split(fullMethod, "/")
	if len(parts) < 3 {
		return true // Default to secure
	}
	protoName := parts[1] + "." + parts[2]

	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(protoName))
	if err != nil {
		return true // Default to secure if descriptor not found
	}

	methodDesc, ok := desc.(protoreflect.MethodDescriptor)
	if !ok {
		return true
	}

	opts := methodDesc.Options()
	if !proto.HasExtension(opts, options.E_Openapiv2Operation) {
		return true // Default to secure (inherits service-level BearerAuth)
	}

	ext := proto.GetExtension(opts, options.E_Openapiv2Operation)
	op, ok := ext.(*options.Operation)
	if !ok || op == nil {
		return true
	}

	// If security is not explicitly set on the operation level, inherit service-level security
	if op.Security == nil {
		return true
	}

	// If security requirement list is empty (e.g. security: {}), it's a public bypass
	if len(op.Security) == 0 {
		return false
	}

	for _, req := range op.Security {
		if req.SecurityRequirement == nil || len(req.SecurityRequirement) == 0 {
			return false // Public bypass explicitly set as security: {}
		}
		if _, exists := req.SecurityRequirement["BearerAuth"]; exists {
			return true // Requires BearerAuth explicitly
		}
	}

	return true
}

// extractToken parses Bearer token from authorization header in gRPC metadata.
func extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errMissingMetadata
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 || authHeaders[0] == "" {
		return "", errMissingAuthHeader
	}

	parts := strings.SplitN(authHeaders[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errInvalidAuthFormat
	}

	return parts[1], nil
}
