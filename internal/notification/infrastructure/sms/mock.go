package sms

import (
	"context"
	"sync"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

var _ port.SMSProvider = (*MockSMSProvider)(nil)

type SentSMS struct {
	To      string
	Message string
}

type MockSMSProvider struct {
	mu          sync.Mutex
	sentMessage []SentSMS
	sendErr     error
}

func NewMockSMSProvider() *MockSMSProvider {
	return &MockSMSProvider{
		sentMessage: make([]SentSMS, 0),
	}
}

func (m *MockSMSProvider) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

func (m *MockSMSProvider) SendSMS(_ context.Context, toPhoneNumber, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendErr != nil {
		return m.sendErr
	}

	m.sentMessage = append(m.sentMessage, SentSMS{
		To:      toPhoneNumber,
		Message: message,
	})
	return nil
}

func (m *MockSMSProvider) GetSentMessages() []SentSMS {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]SentSMS, len(m.sentMessage))
	copy(result, m.sentMessage)
	return result
}

func (m *MockSMSProvider) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessage = m.sentMessage[:0]
}
