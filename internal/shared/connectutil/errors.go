package connectutil

import (
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/grpc/status"
)

// FromGRPCError converts a gRPC status error to a connect error.
// If err is nil, it returns nil.
func FromGRPCError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewError(connect.Code(st.Code()), errors.New(st.Message()))
}
