package fcm

import (
	"context"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
	"google.golang.org/api/option"
)

var _ port.PushProvider = (*Client)(nil)

type Client struct {
	messagingClient *messaging.Client
}

func findServiceAccountFile(specifiedPath string) string {
	if specifiedPath != "" {
		if _, err := os.Stat(specifiedPath); err == nil {
			return specifiedPath
		}
	}
	return ""
}

func NewClient(cfg config.Config) *Client {
	ctx := context.Background()

	var opts []option.ClientOption
	credFile := findServiceAccountFile(cfg.FCMServiceAccountFile)
	if credFile != "" {
		opts = append(opts, option.WithCredentialsFile(credFile))
	} else if cfg.FCMServiceAccountFile != "" {
		log.Printf("⚠️ Warning: FCM service account file '%s' not found or unreadable.", cfg.FCMServiceAccountFile)
	}

	var fbCfg *firebase.Config
	if cfg.FCMProjectID != "" {
		fbCfg = &firebase.Config{ProjectID: cfg.FCMProjectID}
	}

	if len(opts) == 0 && cfg.FCMProjectID == "" {
		log.Println("⚠️ Warning: FCMProjectID and FCMServiceAccountFile are empty. FCM Push SDK disabled.")
		return &Client{messagingClient: nil}
	}

	app, err := firebase.NewApp(ctx, fbCfg, opts...)
	if err != nil {
		log.Printf("❌ Warning: failed to initialize Firebase App: %v", err)
		return &Client{messagingClient: nil}
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("❌ Warning: failed to initialize Firebase Messaging client: %v", err)
		return &Client{messagingClient: nil}
	}

	log.Printf("✅ FCM Firebase Real App & Messaging Client initialized successfully (Project: %s)", cfg.FCMProjectID)
	return &Client{
		messagingClient: msgClient,
	}
}

func (c *Client) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*port.PushResponse, error) {
	if len(tokens) == 0 {
		return &port.PushResponse{SuccessCount: 0, FailureCount: 0}, nil
	}

	if c.messagingClient == nil {
		log.Printf("[FCM PUSH SDK DISABLED] Target Tokens (%d): %v | Title: '%s' | Body: '%s'",
			len(tokens), tokens, title, body)

		return &port.PushResponse{
			SuccessCount:  0,
			FailureCount:  len(tokens),
			InvalidTokens: nil,
		}, nil
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	br, err := c.messagingClient.SendEachForMulticast(ctx, message)
	if err != nil {
		return nil, err
	}

	var invalidTokens []string
	for i, resp := range br.Responses {
		if !resp.Success && resp.Error != nil {
			if messaging.IsUnregistered(resp.Error) || messaging.IsInvalidArgument(resp.Error) {
				if i < len(tokens) {
					invalidTokens = append(invalidTokens, tokens[i])
				}
			}
		}
	}

	return &port.PushResponse{
		SuccessCount:  br.SuccessCount,
		FailureCount:  br.FailureCount,
		InvalidTokens: invalidTokens,
	}, nil
}
