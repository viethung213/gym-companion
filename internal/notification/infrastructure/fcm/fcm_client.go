package fcm

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

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

type serviceAccountJSON struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id,omitempty"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id,omitempty"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	UniverseDomain          string `json:"universe_domain,omitempty"`
}

func buildServiceAccountJSON(cfg *config.Config) ([]byte, error) {
	if cfg == nil || cfg.FCMClientEmail == "" || cfg.FCMPrivateKey == "" {
		return nil, errors.New("missing FCM client email or private key")
	}

	privateKey := strings.ReplaceAll(cfg.FCMPrivateKey, "\\n", "\n")

	sa := serviceAccountJSON{
		Type:                    "service_account",
		ProjectID:               cfg.FCMProjectID,
		PrivateKeyID:            cfg.FCMPrivateKeyID,
		PrivateKey:              privateKey,
		ClientEmail:             cfg.FCMClientEmail,
		ClientID:                cfg.FCMClientID,
		AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
		TokenURI:                "https://oauth2.googleapis.com/token",
		AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
		UniverseDomain:          "googleapis.com",
	}

	return json.Marshal(sa)
}

func NewClient(cfg *config.Config) *Client {
	ctx := context.Background()

	if cfg == nil {
		slog.Warn("Config is nil. FCM Push SDK disabled.")
		return &Client{messagingClient: nil}
	}

	var opts []option.ClientOption
	if saJSON, err := buildServiceAccountJSON(cfg); err == nil {
		opts = append(opts, option.WithCredentialsJSON(saJSON))
	} else if cfg.FCMClientEmail != "" || cfg.FCMPrivateKey != "" {
		slog.Warn("Failed to construct FCM service account JSON", slog.Any("error", err))
	}

	var fbCfg *firebase.Config
	if cfg.FCMProjectID != "" {
		fbCfg = &firebase.Config{ProjectID: cfg.FCMProjectID}
	}

	if len(opts) == 0 && cfg.FCMProjectID == "" {
		slog.Warn("FCM credentials and FCMProjectID are empty. FCM Push SDK disabled.")
		return &Client{messagingClient: nil}
	}

	app, err := firebase.NewApp(ctx, fbCfg, opts...)
	if err != nil {
		slog.Warn("Failed to initialize Firebase App", slog.Any("error", err))
		return &Client{messagingClient: nil}
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		slog.Warn("Failed to initialize Firebase Messaging client", slog.Any("error", err))
		return &Client{messagingClient: nil}
	}

	slog.Info("FCM Firebase Real App & Messaging Client initialized successfully", slog.String("project_id", cfg.FCMProjectID))
	return &Client{
		messagingClient: msgClient,
	}
}

func (c *Client) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*port.PushResponse, error) {
	if len(tokens) == 0 {
		return &port.PushResponse{SuccessCount: 0, FailureCount: 0}, nil
	}

	if c.messagingClient == nil {
		slog.InfoContext(ctx, "FCM Push SDK disabled (dry run)",
			slog.Int("token_count", len(tokens)),
			slog.String("title", title),
			slog.String("body", body))

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
