package config

import "os"

// Config holds all environmental parameters dedicated to the Profile module.
type Config struct {
	KafkaBrokers string
}

// LoadConfig loads environment variables for the Profile module with default fallbacks.
func LoadConfig() Config {
	brokersStr := os.Getenv("KAFKA_BROKERS")
	if brokersStr == "" {
		brokersStr = "localhost:9092"
	}

	return Config{
		KafkaBrokers: brokersStr,
	}
}
