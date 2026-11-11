package gamification_test

import (
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/viethung213/gym-companion/internal/gamification"
	gamificationGRPC "github.com/viethung213/gym-companion/internal/gamification/transport/grpc"
)

func TestRegisterConnectHandler(t *testing.T) {
	server := connect.NewServer()
	handler := &gamificationGRPC.Handler{}

	// Should not panic when registering handler on server
	assert.NotPanics(t, func() {
		gamification.RegisterConnectHandler(server, handler)
	})
}
