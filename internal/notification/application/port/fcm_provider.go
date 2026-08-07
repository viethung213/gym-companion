package port

import "context"

type PushResponse struct {
	SuccessCount  int
	FailureCount  int
	InvalidTokens []string
}

type PushProvider interface {
	SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*PushResponse, error)
}
