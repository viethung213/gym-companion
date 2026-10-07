package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestItemType(t *testing.T) {
	tests := []struct {
		name string
		give string
		want vo.ItemType
	}{
		{
			name: "post item type",
			give: "POST",
			want: vo.ItemTypePost,
		},
		{
			name: "workout activity item type",
			give: "WORKOUT_ACTIVITY",
			want: vo.ItemTypeWorkoutActivity,
		},
		{
			name: "unknown fallback to post",
			give: "UNKNOWN_TYPE",
			want: vo.ItemTypePost,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := vo.NewItemType(tt.give)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if got.String() != string(tt.want) {
				t.Fatalf("got %q, want %q", got.String(), string(tt.want))
			}
		})
	}
}
