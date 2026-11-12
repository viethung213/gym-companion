package port

import "context"

// TrendingPoolCache defines operations for caching the global trending post IDs.
type TrendingPoolCache interface {
	SaveTrendingPool(ctx context.Context, itemIDs []string) error
	GetTrendingPool(ctx context.Context) ([]string, error)
}
