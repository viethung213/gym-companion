package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
)

// S3StorageConfig holds configuration for AWS S3 / Supabase Cloud Storage.

type S3StorageConfig struct {
	BucketName string

	Region string

	AccessKey string

	SecretKey string

	CDNDomain string

	Endpoint string
}

// S3StorageProvider implements port.ObjectStorageProvider using AWS SDK v2.

type S3StorageProvider struct {
	config S3StorageConfig

	client *s3.Client

	presignClient *s3.PresignClient
}

var _ port.ObjectStorageProvider = (*S3StorageProvider)(nil)

// NewS3StorageProviderFromEnv constructs S3StorageProvider reading environment variables.

func NewS3StorageProviderFromEnv() *S3StorageProvider {

	bucket := os.Getenv("AWS_S3_BUCKET")

	if bucket == "" {

		bucket = "gym-companion-assets"

	}

	region := os.Getenv("AWS_REGION")

	// Supabase S3 storage gateway requires 'us-east-1' for SigV4 signature calculations

	if region == "" || strings.Contains(os.Getenv("AWS_S3_ENDPOINT"), "supabase.co") {

		region = "us-east-1"

	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")

	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	endpoint := os.Getenv("AWS_S3_ENDPOINT")

	cdn := deriveCDNDomain(endpoint, bucket)

	cfg := S3StorageConfig{

		BucketName: bucket,

		Region: region,

		AccessKey: accessKey,

		SecretKey: secretKey,

		CDNDomain: cdn,

		Endpoint: endpoint,
	}

	return NewS3StorageProvider(cfg)

}

// NewS3StorageProvider constructs S3StorageProvider with given config.

func NewS3StorageProvider(cfg S3StorageConfig) *S3StorageProvider {

	opts := s3.Options{

		Region: cfg.Region,

		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),

		UsePathStyle: true,
	}

	if cfg.Endpoint != "" {

		opts.BaseEndpoint = aws.String(cfg.Endpoint)

	}

	client := s3.New(opts)

	presignClient := s3.NewPresignClient(client)

	return &S3StorageProvider{

		config: cfg,

		client: client,

		presignClient: presignClient,
	}

}

// deriveCDNDomain computes the public CDN base URL from a Supabase/S3 endpoint.

// For Supabase: strips "/s3" suffix and appends "/object/public/<bucket>".

// For generic S3 endpoints, falls back to a simple CDN convention.

func deriveCDNDomain(endpoint, bucket string) string {

	if endpoint == "" {

		return "https://cdn.fitai.com"

	}

	base := strings.TrimSuffix(strings.TrimSuffix(endpoint, "/"), "/s3")

	return fmt.Sprintf("%s/object/public/%s", base, bucket)

}

// GeneratePresignedUploadURL generates a valid AWS SigV4 presigned URL allowing the Client to upload files directly.

func (p *S3StorageProvider) GeneratePresignedUploadURL(ctx context.Context, fileName, contentType string) (string, string, error) {

	if fileName == "" {

		return "", "", fmt.Errorf("s3 presigned url: file name cannot be empty")

	}

	objectKey := strings.TrimPrefix(path.Clean(fileName), "/")

	publicFileURL := fmt.Sprintf("%s/%s", strings.TrimSuffix(p.config.CDNDomain, "/"), objectKey)

	// Build PutObjectInput for presigning (omit ContentType to avoid client header-mismatch SignatureDoesNotMatch errors)

	input := &s3.PutObjectInput{

		Bucket: aws.String(p.config.BucketName),

		Key: aws.String(objectKey),
	}

	presignedReq, err := p.presignClient.PresignPutObject(ctx, input, s3.WithPresignExpires(1*time.Hour))

	if err != nil {

		return "", "", fmt.Errorf("failed to generate presigned upload url: %w", err)

	}

	return presignedReq.URL, publicFileURL, nil

}

// GetObject fetches object bytes from S3/Supabase storage by key.

func (p *S3StorageProvider) GetObject(ctx context.Context, objectKey string) ([]byte, error) {

	key := strings.TrimPrefix(path.Clean(objectKey), "/")

	if key == "" {

		return nil, fmt.Errorf("s3 get object: object key is required")

	}

	out, err := p.client.GetObject(ctx, &s3.GetObjectInput{

		Bucket: aws.String(p.config.BucketName),

		Key: aws.String(key),
	})

	if err != nil {

		return nil, fmt.Errorf("s3 get object (%s): %w", key, err)

	}

	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)

	if err != nil {

		return nil, fmt.Errorf("s3 read object body (%s): %w", key, err)

	}

	return data, nil

}

// PutObject uploads object bytes directly to S3/Supabase storage and returns public URL.

func (p *S3StorageProvider) PutObject(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {

	key := strings.TrimPrefix(path.Clean(objectKey), "/")

	if key == "" {

		return "", fmt.Errorf("s3 put object: object key is required")

	}

	if contentType == "" {

		contentType = "application/json"

	}

	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{

		Bucket: aws.String(p.config.BucketName),

		Key: aws.String(key),

		Body: bytes.NewReader(data),

		ContentType: aws.String(contentType),
	})

	if err != nil {

		return "", fmt.Errorf("s3 put object (%s): %w", key, err)

	}

	publicFileURL := fmt.Sprintf("%s/%s", strings.TrimSuffix(p.config.CDNDomain, "/"), key)

	return publicFileURL, nil

}
