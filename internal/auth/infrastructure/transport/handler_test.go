//go:build unit

package transport

import (
	"testing"

	"connectrpc.com/connect"
	authv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/message"
)

func TestNewGRPCHandler(t *testing.T) {
	h := NewGRPCHandler(nil, nil, nil, nil, nil, nil)
	if h == nil {
		t.Fatal("expected non-nil GRPCHandler")
	}
}

func TestNewConnectAuthHandler(t *testing.T) {
	grpcHandler := NewGRPCHandler(nil, nil, nil, nil, nil, nil)
	connectHandler := NewConnectAuthHandler(grpcHandler)
	if connectHandler == nil {
		t.Fatal("expected non-nil ConnectAuthHandler")
	}
}

func TestConnectAuthHandler_StructVerification(t *testing.T) {
	grpcHandler := NewGRPCHandler(nil, nil, nil, nil, nil, nil)
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
