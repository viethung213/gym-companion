package service_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/social/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/social/domain/service"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func makeTestItem(id string, userID string, offsetSeconds int) *aggregate.FeedItem {
	t := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC).Add(-time.Duration(offsetSeconds) * time.Second)
	item, _ := aggregate.NewPostItem(id, userID, "content "+id, nil, vo.VisibilityPublic, 0, 0, t, t)
	return item
}

func TestFeedDiversityService_ReorderAndDiversify(t *testing.T) {
	svc := service.NewFeedDiversityService()

	tests := []struct {
		name       string
		giveItems  []*aggregate.FeedItem
		giveCfg    service.FeedDiversityConfig
		wantIDList []string
	}{
		{
			name:       "nil candidates returns nil",
			giveItems:  nil,
			giveCfg:    service.DefaultFeedDiversityConfig(10),
			wantIDList: nil,
		},
		{
			name: "single author exceeds max consecutive triggers soft fallback",
			giveItems: []*aggregate.FeedItem{
				makeTestItem("a1", "user-A", 1),
				makeTestItem("a2", "user-A", 2),
				makeTestItem("a3", "user-A", 3),
				makeTestItem("a4", "user-A", 4),
			},
			giveCfg: service.FeedDiversityConfig{
				MaxConsecutiveAuthor: 2,
				TargetSize:           4,
			},
			wantIDList: []string{"a1", "a2", "a3", "a4"},
		},
		{
			name: "interleaves monopolized author with other author",
			giveItems: []*aggregate.FeedItem{
				makeTestItem("a1", "user-A", 1),
				makeTestItem("a2", "user-A", 2),
				makeTestItem("a3", "user-A", 3),
				makeTestItem("a4", "user-A", 4),
				makeTestItem("b1", "user-B", 5),
				makeTestItem("c1", "user-C", 6),
			},
			giveCfg: service.FeedDiversityConfig{
				MaxConsecutiveAuthor: 2,
				TargetSize:           5,
			},
			// a1, a2 -> hit 2; a3, a4 deferred; b1 selected -> author changed to B -> drain a3, a4 -> total 5
			wantIDList: []string{"a1", "a2", "b1", "a3", "a4"},
		},
		{
			name: "alternating authors naturally preserved",
			giveItems: []*aggregate.FeedItem{
				makeTestItem("a1", "user-A", 1),
				makeTestItem("b1", "user-B", 2),
				makeTestItem("a2", "user-A", 3),
				makeTestItem("b2", "user-B", 4),
			},
			giveCfg: service.FeedDiversityConfig{
				MaxConsecutiveAuthor: 2,
				TargetSize:           4,
			},
			wantIDList: []string{"a1", "b1", "a2", "b2"},
		},
		{
			name: "target size strictly respected when candidate pool is large",
			giveItems: []*aggregate.FeedItem{
				makeTestItem("a1", "user-A", 1),
				makeTestItem("b1", "user-B", 2),
				makeTestItem("c1", "user-C", 3),
				makeTestItem("d1", "user-D", 4),
			},
			giveCfg: service.FeedDiversityConfig{
				MaxConsecutiveAuthor: 2,
				TargetSize:           2,
			},
			wantIDList: []string{"a1", "b1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.ReorderAndDiversify(tt.giveItems, tt.giveCfg)

			if len(got) != len(tt.wantIDList) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.wantIDList))
			}

			for i, item := range got {
				if item.ID() != tt.wantIDList[i] {
					t.Errorf("at index %s: got ID %s, want ID %s",
						strconv.Itoa(i), item.ID(), tt.wantIDList[i])
				}
			}
		})
	}
}
