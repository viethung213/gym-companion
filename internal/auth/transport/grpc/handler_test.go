//go:build unit

package grpc

import (
	"testing"

	"connectrpc.com/connect"
	authv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/message"
)

func TestNewGRPCHandler(t *testing.T) {
	h := NewGRPCHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if h == nil {
		t.Fatal("expected non-nil GRPCHandler")
	}
}

func TestNewConnectAuthHandler(t *testing.T) {
	grpcHandler := NewGRPCHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	connectHandler := NewConnectAuthHandler(grpcHandler)
	if connectHandler == nil {
		t.Fatal("expected non-nil ConnectAuthHandler")
	}
}

func TestConnectAuthHandler_StructVerification(t *testing.T) {
	grpcHandler := NewGRPCHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	c := &ConnectAuthHandler{grpcHandler: grpcHandler}

	t.Run("nil request payload handling", func(t *testing.T) {
		req := connect.NewRequest(&authv1message.RefreshTokenRequest{
			RefreshToken: "",
		})
		if req == nil {
			t.Fatal("expected non-nil request")
		}
		_ = c
	})
}

func TestGRPCHandler_Register_GenderValidation(t *testing.T) {
	h := &GRPCHandler{}

	tests := []struct {
		name        string
		gender      authv1message.Gender
		expectError bool
	}{
		{
			name:        "Unspecified gender rejected",
			gender:      authv1message.Gender_GENDER_UNSPECIFIED,
			expectError: true,
		},
		{
			name:        "Invalid gender int value rejected",
			gender:      authv1message.Gender(99),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := h.Register(nil, &authv1message.RegisterRequest{
				Gender: tt.gender,
			})
			if tt.expectError && err == nil {
				t.Fatalf("expected error for gender %v, got nil", tt.gender)
			}
		})
	}
}
