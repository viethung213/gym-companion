package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/social/application/port"
	"github.com/viethung213/gym-companion/internal/social/domain/repository"
)

// TrendingPoolWorker periodically updates the global trending pool in Redis.
type TrendingPoolWorker struct {
	feedItemRepo  repository.FeedItemRepository
	trendingCache port.TrendingPoolCache
	interval      time.Duration
	poolSize      int
	lookbackHours int
}

// NewTrendingPoolWorker creates a new TrendingPoolWorker.
func NewTrendingPoolWorker(
	feedItemRepo repository.FeedItemRepository,
	trendingCache port.TrendingPoolCache,
	interval time.Duration,
) *TrendingPoolWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &TrendingPoolWorker{
		feedItemRepo:  feedItemRepo,
		trendingCache: trendingCache,
		interval:      interval,
		poolSize:      500,
		lookbackHours: 48,
	}
}

// Start begins periodic refreshing of the trending candidate pool until ctx is cancelled.
func (w *TrendingPoolWorker) Start(ctx context.Context) {
	log.Printf("Starting Social Trending Pool worker (interval: %v)...", w.interval)

	// Warm the cache immediately on startup
	if err := w.Refresh(ctx); err != nil {
		log.Printf("Social trending pool initial warm-up warning: %v", err)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping Social Trending Pool worker due to context cancellation.")
			return
		case <-ticker.C:
			if err := w.Refresh(ctx); err != nil {
				log.Printf("Social trending pool refresh error: %v", err)
			}
		}
	}
}

// Refresh pulls the top trending IDs from persistence and saves them atomically into Redis.
func (w *TrendingPoolWorker) Refresh(ctx context.Context) error {
	ids, err := w.feedItemRepo.GetTrendingCandidateIDs(ctx, w.poolSize, w.lookbackHours)
	if err != nil {
		return fmt.Errorf("fetch trending candidates: %w", err)
	}

	if err := w.trendingCache.SaveTrendingPool(ctx, ids); err != nil {
		return fmt.Errorf("save trending pool to cache: %w", err)
	}

	return nil
}
