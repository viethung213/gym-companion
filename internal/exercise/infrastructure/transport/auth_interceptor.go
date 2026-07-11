package transport

import (
	"context"
	"strings"

	"github.com/viethung213/gym-companion/internal/exercise/application"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func ActorMetadataInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		actor := actorFromMetadata(ctx)
		if actor.UserID != "" {
			ctx = application.ContextWithActor(ctx, actor)
		}

		return handler(ctx, req)
	}
}

func actorFromMetadata(ctx context.Context) application.Actor {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return application.Actor{}
	}

	return application.Actor{
		UserID: firstMetadataValue(md, "x-user-id"),
		Roles:  splitRoles(firstMetadataValue(md, "x-user-roles")),
	}
}

func firstMetadataValue(md metadata.MD, key string) string {
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func splitRoles(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	roles := make([]string, 0, len(parts))
	for _, part := range parts {
		role := strings.TrimSpace(part)
		if role != "" {
			roles = append(roles, role)
		}
	}

	return roles
}
