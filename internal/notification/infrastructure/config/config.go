package config

import "os"

type Config struct {
	FCMProjectID    string
	FCMClientEmail  string
	FCMPrivateKey   string
	FCMPrivateKeyID string
	FCMClientID     string
	FCMServerKey    string
	KafkaBrokers    string
}

func LoadConfig() Config {
	return Config{
		FCMProjectID:    os.Getenv("FCM_PROJECT_ID"),
		FCMClientEmail:  os.Getenv("FCM_CLIENT_EMAIL"),
		FCMPrivateKey:   os.Getenv("FCM_PRIVATE_KEY"),
		FCMPrivateKeyID: os.Getenv("FCM_PRIVATE_KEY_ID"),
		FCMClientID:     os.Getenv("FCM_CLIENT_ID"),
		FCMServerKey:    os.Getenv("FCM_SERVER_KEY"),
		KafkaBrokers:    os.Getenv("KAFKA_BROKERS"),
	}
}
