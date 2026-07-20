package transport

import (
	"context"
	"testing"

	coachingmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCoachingServerInitiateRoadmapAuthorization(t *testing.T) {
	t.Parallel()

	server := NewCoachingServer(nil)
	tests := []struct {
		name string
		ctx  context.Context
		want codes.Code
	}{
		{
			name: "missing identity",
			ctx:  context.Background(),
			want: codes.Unauthenticated,
		},
		{
			name: "different authenticated user",
			ctx: context.WithValue(
				context.Background(),
				middleware.UserIDKey,
				"user-2",
			),
			want: codes.PermissionDenied,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := server.InitiateRoadmap(
				tt.ctx,
				&coachingmsg.InitiateRoadmapRequest{UserId: "user-1"},
			)
			if got := status.Code(err); got != tt.want {
				t.Errorf("status got = %s, want = %s", got, tt.want)
			}
		})
	}
}
