package gateway

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestGateway_DynamicConfigHotReload(t *testing.T) {
	t.Parallel()

	// Backend 1 (Port A) bọc bằng h2c để nhận cả HTTP/2 và HTTP/1.1
	var countServerA atomic.Int64
	handlerA := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		countServerA.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("response from server A"))
	})
	serverA := httptest.NewServer(h2c.NewHandler(handlerA, &http2.Server{}))
	defer serverA.Close()

	// Backend 2 (Port B) bọc bằng h2c
	var countServerB atomic.Int64
	handlerB := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		countServerB.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("response from server B"))
	})
	serverB := httptest.NewServer(h2c.NewHandler(handlerB, &http2.Server{}))
	defer serverB.Close()

	// Tạo file config ban đầu CHỈ CÓ Server A
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "gateway_routes.json")
	initialJSON := []byte(`{
		"services": [
			{
				"id": "test-dynamic-service",
				"prefixes": ["/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"],
				"upstreams": ["` + serverA.URL + `"]
			}
		]
	}`)
	require.NoError(t, os.WriteFile(cfgFile, initialJSON, 0o600))

	// Khởi động Gateway với WatchInterval 50ms để test nhanh
	gw, err := NewGateway(Options{
		ConfigPath:     cfgFile,
		KeyProvider:    &mockKeyProvider{},
		InternalSecret: "test-secret-dynamic",
		TokenTTL:       time.Minute,
		WatchInterval:  50 * time.Millisecond,
	})
	require.NoError(t, err)
	defer gw.Close()

	gatewayServer := httptest.NewServer(gw.Handler())
	defer gatewayServer.Close()

	client := gatewayServer.Client()

	// 1. Gửi request đợt 1: Chỉ server A nhận được
	req, _ := http.NewRequest(http.MethodGet, gatewayServer.URL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", nil)
	resp1, err := client.Do(req)
	require.NoError(t, err)
	defer resp1.Body.Close()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	assert.Equal(t, int64(1), countServerA.Load())
	assert.Equal(t, int64(0), countServerB.Load())

	// 2. GIẢ LẬP: Người dùng thêm Port mới (Server B) vào mảng upstreams của service và Save file
	updatedJSON := []byte(`{
		"services": [
			{
				"id": "test-dynamic-service",
				"prefixes": ["/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"],
				"upstreams": [
					"` + serverA.URL + `",
					"` + serverB.URL + `"
				]
			}
		]
	}`)
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.WriteFile(cfgFile, updatedJSON, 0o600))

	// Chờ watcher của Gateway tự động reload trong RAM (150ms)
	time.Sleep(150 * time.Millisecond)

	// 3. Gửi thêm các request tiếp theo: Gateway tự động Round-Robin sang cả Server B!
	for i := 0; i < 4; i++ {
		r, _ := http.NewRequest(http.MethodGet, gatewayServer.URL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", nil)
		resp, doErr := client.Do(r)
		require.NoError(t, doErr)
		resp.Body.Close()
	}

	// Server B đã nhận được request mà KHÔNG HỀ restart Gateway!
	assert.Greater(t, countServerB.Load(), int64(0), "Server B phải nhận được request sau khi hot reload config")
}
