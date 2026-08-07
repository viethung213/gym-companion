package config

import (
	"os"
	"strings"
)

// Config holds all environmental parameters dedicated to the Workout Execution module.
type Config struct {
	KafkaBrokers           string
	S3BucketName           string
	S3Region               string
	S3AccessKey            string
	S3SecretKey            string
	S3Endpoint             string
	DefaultONNXDetectorURL string
	DefaultONNXSkeletonURL string
}

// LoadConfig loads environment variables for the Workout Execution module with default fallbacks.
func LoadConfig() Config {
	brokersStr := os.Getenv("KAFKA_BROKERS")
	if brokersStr == "" {
		brokersStr = "localhost:9092"
	}

	bucket := os.Getenv("AWS_S3_BUCKET")
	if bucket == "" {
		bucket = "gym-companion-assets"
	}

	endpoint := os.Getenv("AWS_S3_ENDPOINT")
	region := os.Getenv("AWS_REGION")
	if region == "" || strings.Contains(endpoint, "supabase.co") {
		region = "us-east-1"
	}

	return Config{
		KafkaBrokers:           brokersStr,
		S3BucketName:           bucket,
		S3Region:               region,
		S3AccessKey:            os.Getenv("AWS_ACCESS_KEY_ID"),
		S3SecretKey:            os.Getenv("AWS_SECRET_ACCESS_KEY"),
		S3Endpoint:             endpoint,
		DefaultONNXDetectorURL: os.Getenv("DEFAULT_ONNX_DETECTOR_URL"),
		DefaultONNXSkeletonURL: os.Getenv("DEFAULT_ONNX_SKELETON_URL"),
	}
}
