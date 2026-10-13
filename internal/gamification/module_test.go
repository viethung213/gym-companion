package gamification_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/viethung213/gym-companion/internal/gamification"
	gamificationGRPC "github.com/viethung213/gym-companion/internal/gamification/transport/grpc"
)

func TestRegisterConnectHandler(t *testing.T) {
	mux := http.NewServeMux()
	handler := &gamificationGRPC.GRPCHandler{}

	// Should not panic when registering handler on multiplexer
	assert.NotPanics(t, func() {
		gamification.RegisterConnectHandler(mux, handler)
	})
}
