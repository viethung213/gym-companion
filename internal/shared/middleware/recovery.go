package middleware

import (
	"context"
	"log"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryRecoveryInterceptor catches panics in unary RPCs, logs them,
// and returns an internal server error.
func UnaryRecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf(
					"PANIC RECOVERED in gRPC unary method %s: %v\nStack trace:\n%s",
					info.FullMethod,
					r,
					debug.Stack(),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// StreamRecoveryInterceptor catches panics in streaming RPCs (including bidirectional streams),
// logs them, and returns an internal server error.
func StreamRecoveryInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf(
					"PANIC RECOVERED in gRPC stream method %s: %v\nStack trace:\n%s",
					info.FullMethod,
					r,
					debug.Stack(),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}
