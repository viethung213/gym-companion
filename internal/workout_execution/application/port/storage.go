package port

import (
	"context"
)

// ObjectStorageProvider defines operations for interacting with S3 Object Storage.
type ObjectStorageProvider interface {
	GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string) (uploadURL string, publicFileURL string, err error)
	GetObject(ctx context.Context, objectKey string) ([]byte, error)
	PutObject(ctx context.Context, objectKey string, data []byte, contentType string) (publicFileURL string, err error)
}
