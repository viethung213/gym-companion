package email

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
)

var _ port.EmailProvider = (*GmailProvider)(nil)

type GmailProvider struct {
	host      string
	port      string
	username  string
	password  string
	fromEmail string
	fromName  string
}

func NewGmailProvider(cfg *config.Config) *GmailProvider {
	if cfg == nil {
		return &GmailProvider{}
	}

	host := cfg.SMTPHost
	if host == "" {
		host = "smtp.gmail.com"
	}
	port := cfg.SMTPPort
	if port == "" {
		port = "587"
	}

	return &GmailProvider{
		host:      host,
		port:      port,
		username:  cfg.SMTPUsername,
		password:  cfg.SMTPPassword,
		fromEmail: cfg.SMTPFromEmail,
		fromName:  cfg.SMTPFromName,
	}
}

func (p *GmailProvider) SendEmail(ctx context.Context, to, subject, body string) error {
	if strings.TrimSpace(to) == "" {
		return errors.New("recipient email cannot be empty")
	}

	if p.username == "" || p.password == "" {
		log.Printf("[Gmail SMTP Disabled] To: %s | Subject: %s | Body: %s", to, subject, body)
		return nil
	}

	fromHeader := p.fromEmail
	if p.fromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", p.fromName, p.fromEmail)
	}

	headers := make(map[string]string)
	headers["From"] = fromHeader
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=\"UTF-8\""

	var msgBuilder strings.Builder
	for k, v := range headers {
		msgBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msgBuilder.WriteString("\r\n")
	msgBuilder.WriteString(body)

	addr := fmt.Sprintf("%s:%s", p.host, p.port)
	auth := smtp.PlainAuth("", p.username, p.password, p.host)

	// Context cancellation check before sending
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before sending email: %w", err)
	}

	err := smtp.SendMail(addr, auth, p.fromEmail, []string{to}, []byte(msgBuilder.String()))
	if err != nil {
		return fmt.Errorf("send email via smtp (%s): %w", addr, err)
	}

	return nil
}
