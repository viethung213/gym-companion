package config

import "os"

type Config struct {
	FCMProjectID          string
	FCMServiceAccountFile string
	FCMServerKey          string
	KafkaBrokers          string
}

func LoadConfig() Config {
	fcmFile := os.Getenv("FCM_SERVICE_ACCOUNT_FILE")
	if fcmFile == "" {
		candidates := []string{
			"service-account.json",
			"./service-account.json",
			"../service-account.json",
			"/app/service-account.json",
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				fcmFile = cand
				break
			}
		}
	}

	return Config{
		FCMProjectID:          os.Getenv("FCM_PROJECT_ID"),
		FCMServiceAccountFile: fcmFile,
		FCMServerKey:          os.Getenv("FCM_SERVER_KEY"),
		KafkaBrokers:          os.Getenv("KAFKA_BROKERS"),
	}
}
