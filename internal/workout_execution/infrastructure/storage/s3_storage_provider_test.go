package storage_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/storage"
)

func TestS3StorageProvider_GeneratePresignedUploadURL(t *testing.T) {
	cfg := storage.S3StorageConfig{
		BucketName: "test-bucket",
		Region:     "ap-southeast-1",
		AccessKey:  "test-access-key",
		SecretKey:  "test-secret-key",
		Endpoint:   "https://test-ref.supabase.co/storage/v1/s3",
		CDNDomain:  "https://test-ref.supabase.co/storage/v1/object/public/test-bucket",
	}

	provider := storage.NewS3StorageProvider(cfg)

	t.Run("empty filename returns error", func(t *testing.T) {
		_, _, err := provider.GeneratePresignedUploadURL(context.Background(), "", "application/octet-stream")
		if err == nil {
			t.Error("want error on empty filename, got nil")
		}
	})

	t.Run("simple filename in models directory", func(t *testing.T) {
		uploadURL, fileURL, err := provider.GeneratePresignedUploadURL(context.Background(), "models/squat.onnx", "application/octet-stream")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(uploadURL, "models/squat.onnx") {
			t.Errorf("got uploadURL = %s, want to contain models/squat.onnx", uploadURL)
		}
		expectedFileURL := "https://test-ref.supabase.co/storage/v1/object/public/test-bucket/models/squat.onnx"
		if fileURL != expectedFileURL {
			t.Errorf("got fileURL = %s, want %s", fileURL, expectedFileURL)
		}
	})

	t.Run("nested exercise subdirectory", func(t *testing.T) {
		uploadURL, fileURL, err := provider.GeneratePresignedUploadURL(context.Background(), "models/ex-123/onnx/v1.onnx", "application/octet-stream")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(uploadURL, "models/ex-123/onnx/v1.onnx") {
			t.Errorf("got uploadURL = %s, want to contain models/ex-123/onnx/v1.onnx", uploadURL)
		}
		expectedFileURL := "https://test-ref.supabase.co/storage/v1/object/public/test-bucket/models/ex-123/onnx/v1.onnx"
		if fileURL != expectedFileURL {
			t.Errorf("got fileURL = %s, want %s", fileURL, expectedFileURL)
		}
	})

	t.Run("filename already containing models/ prefix does not duplicate models/", func(t *testing.T) {
		uploadURL, fileURL, err := provider.GeneratePresignedUploadURL(context.Background(), "models/rules/ex-456.json", "application/json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.Contains(uploadURL, "models/models/") {
			t.Errorf("uploadURL contains duplicate models/ prefix: %s", uploadURL)
		}
		expectedFileURL := "https://test-ref.supabase.co/storage/v1/object/public/test-bucket/models/rules/ex-456.json"
		if fileURL != expectedFileURL {
			t.Errorf("got fileURL = %s, want %s", fileURL, expectedFileURL)
		}
	})

	t.Run("filename starting with rules/ prefix", func(t *testing.T) {
		_, fileURL, err := provider.GeneratePresignedUploadURL(context.Background(), "rules/ex-789/rule.json", "application/json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedFileURL := "https://test-ref.supabase.co/storage/v1/object/public/test-bucket/rules/ex-789/rule.json"
		if fileURL != expectedFileURL {
			t.Errorf("got fileURL = %s, want %s", fileURL, expectedFileURL)
		}
	})

	t.Run("GetObject empty objectKey returns error", func(t *testing.T) {
		_, err := provider.GetObject(context.Background(), "")
		if err == nil {
			t.Error("want error on empty object key, got nil")
		}
	})

	t.Run("PutObject empty objectKey returns error", func(t *testing.T) {
		_, err := provider.PutObject(context.Background(), "", []byte("test"), "application/json")
		if err == nil {
			t.Error("want error on empty object key, got nil")
		}
	})

	t.Run("GetObject fallback HTTP succeeds when S3 lacks object", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"fallback": true}`))
		}))
		defer ts.Close()

		data, err := provider.GetObject(context.Background(), "non_existent.json", ts.URL+"/rules.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != `{"fallback": true}` {
			t.Errorf("got data = %s, want %s", string(data), `{"fallback": true}`)
		}
	})

	t.Run("GetObject fallback HTTP respects timeout when server hangs", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"slow": true}`))
		}))
		defer slowServer.Close()

		customClient := &http.Client{Timeout: 50 * time.Millisecond}
		providerWithTimeout := storage.NewS3StorageProvider(cfg).WithHTTPClient(customClient)

		start := time.Now()
		_, err := providerWithTimeout.GetObject(context.Background(), "non_existent.json", slowServer.URL+"/rules.json")
		elapsed := time.Since(start)

		if err == nil {
			t.Error("want error on timeout, got nil")
		}
		if elapsed > 150*time.Millisecond {
			t.Errorf("GetObject took %v, expected < 150ms due to timeout", elapsed)
		}
	})
}
