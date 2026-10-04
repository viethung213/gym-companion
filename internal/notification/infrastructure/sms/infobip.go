package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
)

var _ port.SMSProvider = (*InfobipProvider)(nil)

type InfobipProvider struct {
	baseURL    string
	apiKey     string
	from       string
	httpClient *http.Client
}

func NewInfobipProvider(cfg *config.Config) *InfobipProvider {
	if cfg == nil {
		return &InfobipProvider{
			httpClient: &http.Client{Timeout: 10 * time.Second},
		}
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.InfobipBaseURL), "/")
	if baseURL != "" && !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	return &InfobipProvider{
		baseURL:    baseURL,
		apiKey:     cfg.InfobipAPIKey,
		from:       cfg.InfobipFrom,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type infobipDestination struct {
	To string `json:"to"`
}

type infobipMessage struct {
	Destinations []infobipDestination `json:"destinations"`
	From         string               `json:"from,omitempty"`
	Text         string               `json:"text"`
}

type infobipPayload struct {
	Messages []infobipMessage `json:"messages"`
}

func (p *InfobipProvider) SendSMS(ctx context.Context, toPhoneNumber, message string) error {
	if strings.TrimSpace(toPhoneNumber) == "" {
		return errors.New("recipient phone number cannot be empty")
	}

	if p.baseURL == "" || p.apiKey == "" {
		log.Printf("[Infobip SMS Disabled] To: %s | Message: %s", toPhoneNumber, message)
		return nil
	}

	apiURL := p.baseURL + "/sms/2/text/advanced"

	payload := infobipPayload{
		Messages: []infobipMessage{
			{
				Destinations: []infobipDestination{{To: toPhoneNumber}},
				From:         p.from,
				Text:         message,
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal infobip payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create infobip request: %w", err)
	}

	req.Header.Set("Authorization", "App "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch infobip request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("infobip api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
