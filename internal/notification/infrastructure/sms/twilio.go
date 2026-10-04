package sms

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
)

var _ port.SMSProvider = (*TwilioProvider)(nil)

type TwilioProvider struct {
	accountSID string
	authToken  string
	fromNumber string
	httpClient *http.Client
}

func NewTwilioProvider(cfg *config.Config) *TwilioProvider {
	if cfg == nil {
		return &TwilioProvider{
			httpClient: &http.Client{Timeout: 10 * time.Second},
		}
	}

	return &TwilioProvider{
		accountSID: cfg.TwilioAccountSID,
		authToken:  cfg.TwilioAuthToken,
		fromNumber: cfg.TwilioFromNumber,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *TwilioProvider) SendSMS(ctx context.Context, toPhoneNumber, message string) error {
	if strings.TrimSpace(toPhoneNumber) == "" {
		return errors.New("recipient phone number cannot be empty")
	}

	if p.accountSID == "" || p.authToken == "" {
		log.Printf("[Twilio SMS Disabled] To: %s | Message: %s", toPhoneNumber, message)
		return nil
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", p.accountSID)

	data := url.Values{}
	data.Set("To", toPhoneNumber)
	data.Set("From", p.fromNumber)
	data.Set("Body", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create twilio request: %w", err)
	}

	req.SetBasicAuth(p.accountSID, p.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch twilio request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio api error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
