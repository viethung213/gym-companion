package middleware

import (
	"context"
	"fmt"
	"strings"

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

var keyProvider KeyProvider

// SetKeyProvider registers the KeyProvider implementation for the auth interceptors.
func SetKeyProvider(kp KeyProvider) {
	keyProvider = kp
}

// UnaryAuthInterceptor intercepts unary gRPC requests to validate JWT tokens.
func UnaryAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// 1. Check if method bypasses auth based on proto contract annotations
		required := isAuthRequired(info.FullMethod)

		// 2. Extract token from metadata
		tokenStr, err := extractToken(ctx)
		if err != nil {
			if required {
				return nil, status.Errorf(codes.Unauthenticated, "authentication required: %v", err)
			}
			// Bypass if not required
			return handler(ctx, req)
		}

		// 3. Validate token
		userID, roles, err := parseAndValidateToken(ctx, tokenStr)
		if err != nil {
			if required {
				return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
			}
			// Bypass if not required
			return handler(ctx, req)
		}

		// 4. Inject identity into context
		ctx = context.WithValue(ctx, "userId", userID)
		ctx = context.WithValue(ctx, "userRoles", roles)

		return handler(ctx, req)
	}
}


func parseAndValidateToken(ctx context.Context, tokenStr string) (string, []string, error) {
	if keyProvider == nil {
		return "", nil, fmt.Errorf("key provider not initialized")
	}

	var kid string
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		var ok bool
		kid, ok = token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("missing kid in token header")
		}

		pubKeyPEM, err := keyProvider.GetPublicKeyPEM(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("get public key: %w", err)
		}

		pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pubKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("parse rsa public key: %w", err)
		}

		return pubKey, nil
	})

	if err != nil {
		return "", nil, err
	}

	if !token.Valid {
		return "", nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", nil, fmt.Errorf("invalid claims")
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return "", nil, fmt.Errorf("missing sub claim")
	}

	var roles []string
	if rs, ok := claims["roles"].([]interface{}); ok {
		for _, r := range rs {
			if str, ok := r.(string); ok {
				roles = append(roles, str)
			}
		}
	}

	return userID, roles, nil
}


// isAuthRequired checks the compiled protobuf annotations to see if the method requires BearerAuth.
func isAuthRequired(fullMethod string) bool {
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

	// If security requirement list is empty (e.g. security: {} or security: [])
	if len(op.Security) == 0 {
		return false // Public bypass
	}

	for _, req := range op.Security {
		if req.SecurityRequirement != nil {
			if _, exists := req.SecurityRequirement["BearerAuth"]; exists {
				return true // Requires BearerAuth explicitly
			}
		}
	}

	return false
}

// extractToken parses Bearer token from authorization header in gRPC metadata.
func extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("missing metadata")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 || authHeaders[0] == "" {
		return "", fmt.Errorf("missing authorization header")
	}

	parts := strings.SplitN(authHeaders[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", fmt.Errorf("invalid authorization format (expected Bearer <token>)")
	}

	return parts[1], nil
}
