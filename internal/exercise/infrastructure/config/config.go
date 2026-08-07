package config

import "os"

// Config holds all environmental parameters dedicated to the Exercise module.
type Config struct {
	KafkaBrokers string
}

// LoadConfig loads environment variables for the Exercise module with default fallbacks.
func LoadConfig() Config {
	brokersStr := os.Getenv("EXERCISE_KAFKA_BROKERS")
	if brokersStr == "" {
		brokersStr = os.Getenv("KAFKA_BROKERS")
	}
	if brokersStr == "" {
		brokersStr = "localhost:9092"
	}

	return Config{
		KafkaBrokers: brokersStr,
	}
}
