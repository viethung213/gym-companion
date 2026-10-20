package repository

import "context"

// BotRepository định nghĩa cổng xác thực và truy vấn System Bots
type BotRepository interface {
	IsBot(ctx context.Context, userID string) (bool, error)
	GetRandomBotUserID(ctx context.Context) (string, error)
}
