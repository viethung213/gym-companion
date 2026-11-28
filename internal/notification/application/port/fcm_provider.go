package port

import "context"

type PushResponse struct {
	SuccessCount  int
	FailureCount  int
	InvalidTokens []string
}

type PushProvider interface {
	SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) (*PushResponse, error)
	SendPushToTopic(ctx context.Context, topic string, title, body string, data map[string]string) error
	SubscribeToTopic(ctx context.Context, tokens []string, topic string) error
	UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error
}
