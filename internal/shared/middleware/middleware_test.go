package middleware

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestConnectRecoveryInterceptor(t *testing.T) {
	t.Parallel()
	interceptor := NewConnectRecoveryInterceptor()

	// 1. Recovery on panic
	handler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		panic("database crash")
	})

	err := handler(context.Background(), connect.Spec{Procedure: "/test/Panic"}, nil)
	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
	var cErr *connect.Error
	if !errors.As(err, &cErr) || cErr.Code() != connect.CodeInternal {
		t.Errorf("expected connect.CodeInternal, got %v", err)
	}

	// 2. Normal execution
	successHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return nil
	})
	if err := successHandler(context.Background(), connect.Spec{Procedure: "/test/OK"}, nil); err != nil {
		t.Fatalf("expected success, got err=%v", err)
	}
}

func TestConnectLoggingInterceptor(t *testing.T) {
	t.Parallel()
	interceptor := NewConnectLoggingInterceptor()

	// 1. Logging success
	handler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return nil
	})
	if err := handler(context.Background(), connect.Spec{Procedure: "/test/Logging"}, nil); err != nil {
		t.Fatalf("expected success, got err=%v", err)
	}

	// 2. Logging client/business error (warn level)
	clientErrHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return connect.NewError(connect.CodeInvalidArgument, "bad argument")
	})
	if err := clientErrHandler(context.Background(), connect.Spec{Procedure: "/test/LoggingClientErr"}, nil); err == nil {
		t.Fatal("expected error, got nil")
	}

	// 3. Logging server/system error (error level)
	serverErrHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return connect.NewError(connect.CodeInternal, "internal server failure")
	})
	if err := serverErrHandler(context.Background(), connect.Spec{Procedure: "/test/LoggingServerErr"}, nil); err == nil {
		t.Fatal("expected error, got nil")
	}

	// 4. Logging raw unmapped error (error level)
	rawErrHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return errors.New("raw unexpected failure")
	})
	if err := rawErrHandler(context.Background(), connect.Spec{Procedure: "/test/LoggingRawErr"}, nil); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestIsClientError(t *testing.T) {
	t.Parallel()

	clientCodes := []connect.Code{
		connect.CodeCanceled,
		connect.CodeInvalidArgument,
		connect.CodeNotFound,
		connect.CodeAlreadyExists,
		connect.CodePermissionDenied,
		connect.CodeResourceExhausted,
		connect.CodeFailedPrecondition,
		connect.CodeAborted,
		connect.CodeOutOfRange,
		connect.CodeUnauthenticated,
	}

	for _, code := range clientCodes {
		if !IsClientError(code) {
			t.Errorf("expected IsClientError(%v) = true, got false", code)
		}
	}

	serverCodes := []connect.Code{
		connect.CodeUnknown,
		connect.CodeDeadlineExceeded,
		connect.CodeUnimplemented,
		connect.CodeInternal,
		connect.CodeUnavailable,
		connect.CodeDataLoss,
	}

	for _, code := range serverCodes {
		if IsClientError(code) {
			t.Errorf("expected IsClientError(%v) = false, got true", code)
		}
	}
}

func TestRequireAuthenticatedAndAdmin(t *testing.T) {
	t.Parallel()

	// 1. Unauthenticated context
	_, err := RequireAuthenticated(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}

	// 2. Authenticated non-admin
	userCtx := context.WithValue(context.Background(), UserIDKey, "u123")
	userCtx = context.WithValue(userCtx, UserRoleKey, "member")
	actor, err := RequireAuthenticated(userCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actor.UserID != "u123" || actor.Role != "member" {
		t.Errorf("unexpected actor: %+v", actor)
	}
	if actor.IsAdmin() {
		t.Error("expected non-admin")
	}

	if _, err := RequireAdmin(userCtx); !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden for member, got %v", err)
	}

	// 3. Authenticated admin
	adminCtx := context.WithValue(context.Background(), UserIDKey, "admin1")
	adminCtx = context.WithValue(adminCtx, UserRoleKey, "admin")
	adminActor, err := RequireAuthenticated(adminCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !adminActor.IsAdmin() {
		t.Error("expected admin")
	}
	if _, err := RequireAdmin(adminCtx); err != nil {
		t.Errorf("expected nil error for admin, got %v", err)
	}
}

func TestToConnectError(t *testing.T) {
	t.Parallel()

	// 1. Nil error returns nil
	if err := ToConnectError(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// 2. Already *connect.Error returns as-is
	origErr := connect.NewError(connect.CodeNotFound, "user missing")
	if mapped := ToConnectError(origErr); mapped != origErr {
		t.Errorf("expected same instance of *connect.Error, got %v", mapped)
	}

	// 3. Mapping of all gRPC status codes
	testCases := []struct {
		grpcCode codes.Code
		wantCode connect.Code
	}{
		{codes.OK, 0},
		{codes.Canceled, connect.CodeCanceled},
		{codes.Unknown, connect.CodeUnknown},
		{codes.InvalidArgument, connect.CodeInvalidArgument},
		{codes.DeadlineExceeded, connect.CodeDeadlineExceeded},
		{codes.NotFound, connect.CodeNotFound},
		{codes.AlreadyExists, connect.CodeAlreadyExists},
		{codes.PermissionDenied, connect.CodePermissionDenied},
		{codes.ResourceExhausted, connect.CodeResourceExhausted},
		{codes.FailedPrecondition, connect.CodeFailedPrecondition},
		{codes.Aborted, connect.CodeAborted},
		{codes.OutOfRange, connect.CodeOutOfRange},
		{codes.Unimplemented, connect.CodeUnimplemented},
		{codes.Internal, connect.CodeInternal},
		{codes.Unavailable, connect.CodeUnavailable},
		{codes.DataLoss, connect.CodeDataLoss},
		{codes.Unauthenticated, connect.CodeUnauthenticated},
	}

	for _, tc := range testCases {
		gErr := status.Error(tc.grpcCode, "test error message")
		mapped := ToConnectError(gErr)

		if tc.grpcCode == codes.OK {
			if mapped != nil {
				t.Errorf("expected nil for codes.OK, got %v", mapped)
			}
			continue
		}

		var cErr *connect.Error
		if !errors.As(mapped, &cErr) {
			t.Fatalf("expected *connect.Error, got %T for code %v", mapped, tc.grpcCode)
		}
		if cErr.Code() != tc.wantCode {
			t.Errorf("for grpc code %v, expected connect code %v, got %v", tc.grpcCode, tc.wantCode, cErr.Code())
		}
	}

	// 4. Raw error maps to CodeInternal
	rawErr := errors.New("raw unexpected error")
	mappedRaw := ToConnectError(rawErr)
	var cErr *connect.Error
	if !errors.As(mappedRaw, &cErr) {
		t.Fatalf("expected *connect.Error for raw error, got %T", mappedRaw)
	}
	if cErr.Code() != connect.CodeInternal {
		t.Errorf("expected connect.CodeInternal for raw error, got %s", cErr.Code())
	}
}

func TestConnectErrorMappingInterceptor(t *testing.T) {
	t.Parallel()

	interceptor := NewConnectErrorMappingInterceptor()

	noopHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return nil
	})
	if err := noopHandler(context.Background(), connect.Spec{Procedure: "/test/Noop"}, nil); err != nil {
		t.Errorf("expected nil err, got err=%v", err)
	}

	errorHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return status.Error(codes.Unauthenticated, "invalid password")
	})
	err := errorHandler(context.Background(), connect.Spec{Procedure: "/test/AuthErr"}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var cErr *connect.Error
	if !errors.As(err, &cErr) {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if cErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("expected connect.CodeUnauthenticated, got %s", cErr.Code())
	}
}

func TestConnectTracingInterceptor(t *testing.T) {
	t.Parallel()

	interceptor := NewConnectTracingInterceptor("test-service")

	// 1. Success path
	successHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return nil
	})
	if err := successHandler(context.Background(), connect.Spec{Procedure: "/contracts.core.auth.v1.service.AuthService/Login"}, nil); err != nil {
		t.Fatalf("expected success, got err=%v", err)
	}

	// 2. Client error path (should not crash and handles client code)
	clientErrHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return connect.NewError(connect.CodeAlreadyExists, "brand request already pending")
	})
	if err := clientErrHandler(context.Background(), connect.Spec{Procedure: "/contracts.generic.auth.v1.service.AuthService/RegisterBrand"}, nil); err == nil {
		t.Fatal("expected error, got nil")
	}

	// 3. Server error path
	serverErrHandler := interceptor(func(_ context.Context, _ connect.Spec, _ connect.ServerStream) error {
		return connect.NewError(connect.CodeInternal, "db failure")
	})
	if err := serverErrHandler(context.Background(), connect.Spec{Procedure: "/contracts.core.auth.v1.service.AuthService/InternalOp"}, nil); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractModuleFromProcedure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		procedure string
		want      string
	}{
		{"/contracts.core.workout_execution.v1.service.WorkoutExecutionService/StartSession", "workout_execution"},
		{"/contracts.generic.auth.v1.service.AuthService/Login", "auth"},
		{"/short.path", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractModuleFromProcedure(tt.procedure)
		if got != tt.want {
			t.Errorf("extractModuleFromProcedure(%q) = %q, want %q", tt.procedure, got, tt.want)
		}
	}
}
