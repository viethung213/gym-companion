package email

import (
	"context"
	"sync"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

var _ port.EmailProvider = (*MockEmailProvider)(nil)

type SentEmail struct {
	To      string
	Subject string
	Body    string
}

type MockEmailProvider struct {
	mu         sync.Mutex
	sentEmails []SentEmail
	sendErr    error
}

func NewMockEmailProvider() *MockEmailProvider {
	return &MockEmailProvider{
		sentEmails: make([]SentEmail, 0),
	}
}

func (m *MockEmailProvider) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

func (m *MockEmailProvider) SendEmail(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendErr != nil {
		return m.sendErr
	}

	m.sentEmails = append(m.sentEmails, SentEmail{
		To:      to,
		Subject: subject,
		Body:    body,
	})
	return nil
}

func (m *MockEmailProvider) GetSentEmails() []SentEmail {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]SentEmail, len(m.sentEmails))
	copy(result, m.sentEmails)
	return result
}

func (m *MockEmailProvider) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentEmails = m.sentEmails[:0]
}
