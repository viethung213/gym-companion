package storage_test

import (
	"context"
	"strings"
	"testing"

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
}
