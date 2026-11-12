package service

import "github.com/viethung213/gym-companion/internal/social/domain/aggregate"

// FeedDiversityConfig specifies constraints for feed interleaving and author diversity.
type FeedDiversityConfig struct {
	MaxConsecutiveAuthor int
	TargetSize           int
}

// DefaultFeedDiversityConfig returns sensible defaults for feed diversity.
func DefaultFeedDiversityConfig(targetSize int) FeedDiversityConfig {
	if targetSize <= 0 {
		targetSize = 20
	}
	return FeedDiversityConfig{
		MaxConsecutiveAuthor: 2,
		TargetSize:           targetSize,
	}
}

// FeedDiversityService enforces author diversity on feed candidate items.
type FeedDiversityService struct{}

// NewFeedDiversityService creates a new FeedDiversityService.
func NewFeedDiversityService() *FeedDiversityService {
	return &FeedDiversityService{}
}

// ReorderAndDiversify limits consecutive items from the same author to prevent monopolization.
// If not enough unique authors exist to fulfill TargetSize, deferred items are appended (soft fallback).
func (s *FeedDiversityService) ReorderAndDiversify(
	candidates []*aggregate.FeedItem,
	cfg FeedDiversityConfig,
) []*aggregate.FeedItem {
	if len(candidates) == 0 {
		return nil
	}
	if cfg.MaxConsecutiveAuthor <= 0 {
		cfg.MaxConsecutiveAuthor = 2
	}
	if cfg.TargetSize <= 0 {
		cfg.TargetSize = len(candidates)
	}

	selected := make([]*aggregate.FeedItem, 0, min(len(candidates), cfg.TargetSize))
	var deferred []*aggregate.FeedItem
	var lastAuthorID string
	var consecutiveCount int

	for _, item := range candidates {
		if len(selected) >= cfg.TargetSize {
			break
		}

		authorID := item.UserID()
		if authorID == lastAuthorID && consecutiveCount >= cfg.MaxConsecutiveAuthor {
			deferred = append(deferred, item)
			continue
		}

		selected = append(selected, item)
		if authorID == lastAuthorID {
			consecutiveCount++
		} else {
			lastAuthorID = authorID
			consecutiveCount = 1
			deferred = s.drainEligibleDeferred(&selected, deferred, &lastAuthorID, &consecutiveCount, cfg)
		}
	}

	// Soft fallback: If we still need items and deferred has remaining items, append them.
	for _, item := range deferred {
		if len(selected) >= cfg.TargetSize {
			break
		}
		selected = append(selected, item)
	}

	return selected
}

func (s *FeedDiversityService) drainEligibleDeferred(
	selected *[]*aggregate.FeedItem,
	deferred []*aggregate.FeedItem,
	lastAuthorID *string,
	consecutiveCount *int,
	cfg FeedDiversityConfig,
) []*aggregate.FeedItem {
	for len(deferred) > 0 && len(*selected) < cfg.TargetSize {
		eligibleIdx := -1
		for i, item := range deferred {
			authorID := item.UserID()
			if authorID != *lastAuthorID || *consecutiveCount < cfg.MaxConsecutiveAuthor {
				eligibleIdx = i
				break
			}
		}

		if eligibleIdx == -1 {
			break
		}

		itemToPull := deferred[eligibleIdx]
		deferred = append(deferred[:eligibleIdx], deferred[eligibleIdx+1:]...)
		*selected = append(*selected, itemToPull)

		if itemToPull.UserID() == *lastAuthorID {
			*consecutiveCount++
		} else {
			*lastAuthorID = itemToPull.UserID()
			*consecutiveCount = 1
		}
	}

	return deferred
}
