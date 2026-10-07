package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestVisibility(t *testing.T) {
	tests := []struct {
		name string
		give string
		want vo.Visibility
	}{
		{name: "followers only", give: "FOLLOWERS_ONLY", want: vo.VisibilityFollowersOnly},
		{name: "private", give: "PRIVATE", want: vo.VisibilityPrivate},
		{name: "public default", give: "PUBLIC", want: vo.VisibilityPublic},
		{name: "unknown fallback to public", give: "ANYTHING", want: vo.VisibilityPublic},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := vo.NewVisibility(tt.give)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if got.String() != string(tt.want) {
				t.Fatalf("got string %q, want %q", got.String(), string(tt.want))
			}
		})
	}
}
