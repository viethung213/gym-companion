package port

import "context"

type UserPRProvider interface {
	GetBestPR(ctx context.Context, userID, exerciseID string) (*float64, error)
}
