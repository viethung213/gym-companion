package middleware

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestConnectRecoveryInterceptor(t *testing.T) {
	t.Parallel()
	interceptor := NewConnectRecoveryInterceptor()

	// 1. Unary recovery
	unaryPanicHandler := interceptor.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		panic("unary database crash")
	})

	resp, err := unaryPanicHandler(context.Background(), nil)
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
	var cErr *connect.Error
	if !errors.As(err, &cErr) || cErr.Code() != connect.CodeInternal {
		t.Errorf("expected connect.CodeInternal, got %v", err)
	}

	// 2. Unary normal execution
	unarySuccessHandler := interceptor.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	})
	resp, err = unarySuccessHandler(context.Background(), nil)
	if err != nil || resp == nil {
		t.Fatalf("expected success, got resp=%v, err=%v", resp, err)
	}

	// 3. Streaming client wrap passthrough
	dummyClient := interceptor.WrapStreamingClient(func(_ context.Context, _ connect.Spec) connect.StreamingClientConn {
		return nil
	})
	if dummyClient(context.Background(), connect.Spec{}) != nil {
		t.Error("expected nil streaming client")
	}
}

func TestConnectLoggingInterceptor(t *testing.T) {
	t.Parallel()
	interceptor := NewConnectLoggingInterceptor()

	// 1. Unary logging success and error
	unaryHandler := interceptor.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	})
	resp, err := unaryHandler(context.Background(), nil)
	if err != nil || resp == nil {
		t.Fatalf("expected success response, got resp=%v, err=%v", resp, err)
	}

	unaryErrHandler := interceptor.WrapUnary(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("bad argument"))
	})
	_, err = unaryErrHandler(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// 2. Streaming handler logging
	conn := &mockStreamingHandlerConn{
		spec: connect.Spec{Procedure: "/test/LoggingStream"},
	}
	streamHandler := interceptor.WrapStreamingHandler(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		return nil
	})
	if err := streamHandler(context.Background(), conn); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// 3. Streaming client wrap passthrough
	dummyClient := interceptor.WrapStreamingClient(func(_ context.Context, _ connect.Spec) connect.StreamingClientConn {
		return nil
	})
	if dummyClient(context.Background(), connect.Spec{}) != nil {
		t.Error("expected nil streaming client")
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
	userCtx := context.WithValue(context.Background(), UserIDKey, "user-123")
	userCtx = context.WithValue(userCtx, UserRoleKey, "Member")
	actor, err := RequireAuthenticated(userCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actor.UserID != "user-123" || actor.Role != "Member" {
		t.Errorf("unexpected actor: %+v", actor)
	}

	_, err = RequireAdmin(userCtx)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden for non-admin, got %v", err)
	}

	// 3. Admin user
	adminCtx := context.WithValue(context.Background(), UserIDKey, "admin-1")
	adminCtx = context.WithValue(adminCtx, UserRoleKey, "Admin")
	adminActor, err := RequireAdmin(adminCtx)
	if err != nil {
		t.Fatalf("unexpected error for admin: %v", err)
	}
	if !adminActor.IsAdmin() {
		t.Errorf("expected actor to be admin")
	}
}

func TestToConnectError(t *testing.T) {
	t.Parallel()

	// 1. Nil error returns nil
	if err := ToConnectError(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// 2. Already *connect.Error returns as-is
	origCErr := connect.NewError(connect.CodeNotFound, errors.New("not found"))
	if err := ToConnectError(origCErr); err != origCErr {
		t.Errorf("expected identical connect error, got %v", err)
	}

	// 3. gRPC status mapping
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

	noopHandler := interceptor.WrapUnary(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil
	})
	resp, err := noopHandler(context.Background(), nil)
	if err != nil || resp != nil {
		t.Errorf("expected nil resp and nil err, got resp=%v, err=%v", resp, err)
	}

	errorHandler := interceptor.WrapUnary(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, status.Error(codes.Unauthenticated, "invalid password")
	})
	_, err = errorHandler(context.Background(), nil)
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

	// Streaming client wrap passthrough
	dummyClient := interceptor.WrapStreamingClient(func(_ context.Context, _ connect.Spec) connect.StreamingClientConn {
		return nil
	})
	if dummyClient(context.Background(), connect.Spec{}) != nil {
		t.Error("expected nil streaming client")
	}
}

type mockStreamingHandlerConn struct {
	spec        connect.Spec
	peer        connect.Peer
	reqHeader   http.Header
	respHeader  http.Header
	respTrailer http.Header
}

func (m *mockStreamingHandlerConn) Spec() connect.Spec           { return m.spec }
func (m *mockStreamingHandlerConn) Peer() connect.Peer           { return m.peer }
func (m *mockStreamingHandlerConn) Receive(any) error            { return nil }
func (m *mockStreamingHandlerConn) RequestHeader() http.Header   { return m.reqHeader }
func (m *mockStreamingHandlerConn) ResponseHeader() http.Header  { return m.respHeader }
func (m *mockStreamingHandlerConn) ResponseTrailer() http.Header { return m.respTrailer }
func (m *mockStreamingHandlerConn) Send(any) error               { return nil }

func TestConnectStreamingRecoveryAndErrorMapping(t *testing.T) {
	t.Parallel()

	// 1. Recovery
	recInterceptor := NewConnectRecoveryInterceptor()
	panicConn := &mockStreamingHandlerConn{
		spec: connect.Spec{Procedure: "/test/PanicStream"},
	}
	panicHandler := recInterceptor.WrapStreamingHandler(func(ctx context.Context, _ connect.StreamingHandlerConn) error {
		panic("stream explosion")
	})
	err := panicHandler(context.Background(), panicConn)
	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
	var cErr *connect.Error
	if !errors.As(err, &cErr) || cErr.Code() != connect.CodeInternal {
		t.Fatalf("expected CodeInternal for panic, got %v", err)
	}

	// 2. Error Mapping
	errInterceptor := NewConnectErrorMappingInterceptor()
	grpcErrConn := &mockStreamingHandlerConn{
		spec: connect.Spec{Procedure: "/test/ErrStream"},
	}
	grpcErrHandler := errInterceptor.WrapStreamingHandler(func(ctx context.Context, _ connect.StreamingHandlerConn) error {
		return status.Error(codes.NotFound, "stream resource missing")
	})
	err = grpcErrHandler(context.Background(), grpcErrConn)
	if err == nil {
		t.Fatal("expected mapped error, got nil")
	}
	if !errors.As(err, &cErr) || cErr.Code() != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", err)
	}
}
