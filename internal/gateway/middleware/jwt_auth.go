package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// JWTValidator validates incoming public JWTs using RSA public keys.
type JWTValidator struct {
	mu          sync.RWMutex
	keyProvider KeyProvider
	keyCache    map[string]*rsa.PublicKey
}

// NewJWTValidator initializes a JWT validator backed by a KeyProvider.
func NewJWTValidator(kp KeyProvider) *JWTValidator {
	return &JWTValidator{
		keyProvider: kp,
		keyCache:    make(map[string]*rsa.PublicKey),
	}
}

// Verify extracts and verifies user claims from Authorization header.
func (v *JWTValidator) Verify(ctx context.Context, authHeader string) (userID, role string, err error) {
	if authHeader == "" {
		return "", "", errors.New("missing authorization header")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", "", errors.New("invalid authorization format (expected Bearer <token>)")
	}

	tokenStr := parts[1]
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid in token header")
		}

		return v.getPublicKey(ctx, kid)
	})

	if err != nil {
		return "", "", fmt.Errorf("verify jwt: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", errors.New("invalid token claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		sub, _ = claims["userId"].(string)
	}
	roleVal, _ := claims["role"].(string)
	if roleVal == "" {
		roleVal = "Member"
	}

	return sub, roleVal, nil
}

func (v *JWTValidator) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	pubKey, ok := v.keyCache[kid]
	v.mu.RUnlock()
	if ok {
		return pubKey, nil
	}

	// If keyProvider is RemoteJWKSKeyProvider, directly query it
	if rk, isRemote := v.keyProvider.(*RemoteJWKSKeyProvider); isRemote {
		key, err := rk.GetPublicKey(ctx, kid)
		if err != nil {
			return nil, err
		}
		v.mu.Lock()
		v.keyCache[kid] = key
		v.mu.Unlock()
		return key, nil
	}

	// Fallback to PEM parser
	pemStr, err := v.keyProvider.GetPublicKeyPEM(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("fetch public key pem: %w", err)
	}

	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the public key")
	}

	parsedKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse rsa public key: %w", err)
	}

	rsaPub, ok := parsedKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}

	v.mu.Lock()
	v.keyCache[kid] = rsaPub
	v.mu.Unlock()

	return rsaPub, nil
}

// IsAuthRequired checks the compiled protobuf annotations to see if the method requires BearerAuth.
// It is 100% dynamically driven by Proto OpenAPI annotations as the Single Source of Truth (SSOT).
func IsAuthRequired(fullMethod string) bool {
	clean := strings.TrimPrefix(fullMethod, "/")
	parts := strings.Split(clean, "/")
	if len(parts) != 2 {
		return true // Default to secure if malformed
	}

	protoName := parts[0] + "." + parts[1]
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
		return true // Default to secure
	}

	ext := proto.GetExtension(opts, options.E_Openapiv2Operation)
	op, ok := ext.(*options.Operation)
	if !ok || op == nil || op.Security == nil {
		return true
	}

	if len(op.Security) == 0 {
		return false // Public bypass explicitly set as security: {}
	}

	for _, req := range op.Security {
		if req.SecurityRequirement == nil || len(req.SecurityRequirement) == 0 {
			return false // Public bypass
		}
	}

	return true
}
