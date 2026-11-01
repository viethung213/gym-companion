package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestProxyEngine_H2CRouting(t *testing.T) {
	t.Parallel()

	var receivedHost string
	backend := httptest.NewServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}), &http2.Server{}))
	defer backend.Close()

	engine := NewEngine()
	targetURL, err := url.Parse(backend.URL)
	require.NoError(t, err)

	rp := engine.CreateProxy(targetURL)
	proxyServer := httptest.NewServer(rp)
	defer proxyServer.Close()

	resp, err := http.Get(proxyServer.URL + "/test")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, targetURL.Host, receivedHost)
}

func TestProxyEngine_RejectsPlainHTTP1(t *testing.T) {
	t.Parallel()

	// Plain HTTP/1.1 backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"plain http1"}`))
	}))
	defer backend.Close()

	engine := NewEngine()
	targetURL, err := url.Parse(backend.URL)
	require.NoError(t, err)

	rp := engine.CreateProxy(targetURL)
	proxyServer := httptest.NewServer(rp)
	defer proxyServer.Close()

	resp, err := http.Get(proxyServer.URL + "/test")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}
