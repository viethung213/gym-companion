package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestRegistrar_Lifecycle(t *testing.T) {
	var registerCalls atomic.Int32
	var deregisterCalls atomic.Int32

	h2Handler := h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-admin-key", r.Header.Get("X-Admin-Key"))

		var payload struct {
			ServiceID   string   `json:"service_id"`
			UpstreamURL string   `json:"upstream_url"`
			Prefixes    []string `json:"prefixes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		assert.Equal(t, "test-service", payload.ServiceID)
		assert.Equal(t, "http://localhost:1234", payload.UpstreamURL)
		switch r.URL.Path {
		case "/admin/discovery/register":
			assert.Equal(t, []string{"/test.Service/"}, payload.Prefixes)
			registerCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"OK"}`))
		case "/admin/discovery/deregister":
			deregisterCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"OK"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}), &http2.Server{})

	server := httptest.NewServer(h2Handler)
	defer server.Close()

	cfg := Config{
		GatewayAdminURL:   server.URL,
		AdminKey:          "test-admin-key",
		ServiceID:         "test-service",
		ServicePort:       "1234",
		UpstreamURL:       "http://localhost:1234",
		Prefixes:          []string{"/test.Service/"},
		HeartbeatInterval: 30 * time.Millisecond,
	}

	reg := NewRegistrar(&cfg)
	assert.Equal(t, "http://localhost:1234", reg.UpstreamURL())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg.Start(ctx)

	// Wait for 2-3 heartbeat cycles
	time.Sleep(100 * time.Millisecond)

	assert.GreaterOrEqual(t, registerCalls.Load(), int32(2))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	err := reg.Stop(shutdownCtx)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), deregisterCalls.Load())
}
