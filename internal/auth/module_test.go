//go:build unit

package auth

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
	"google.golang.org/grpc"
)

func TestInitialize_NilDependencies(t *testing.T) {
	ctx := context.Background()
	dummyDB := &sql.DB{}
	dummyGRPC := grpc.NewServer()
	dummyKafka := &sharedKafka.Registry{}

	tests := []struct {
		name    string
		deps    ModuleDeps
		wantErr string
	}{
		{
			name: "nil DB",
			deps: ModuleDeps{
				DB:            nil,
				GRPCServer:    dummyGRPC,
				KafkaRegistry: dummyKafka,
			},
			wantErr: "deps.DB is required",
		},
		{
			name: "nil GRPCServer",
			deps: ModuleDeps{
				DB:            dummyDB,
				GRPCServer:    nil,
				KafkaRegistry: dummyKafka,
			},
			wantErr: "deps.GRPCServer is required",
		},
		{
			name: "nil KafkaRegistry",
			deps: ModuleDeps{
				DB:            dummyDB,
				GRPCServer:    dummyGRPC,
				KafkaRegistry: nil,
			},
			wantErr: "deps.KafkaRegistry is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shutdown, err := Initialize(ctx, tt.deps)
			if err == nil {
				if shutdown != nil {
					shutdown()
				}
				t.Fatalf("Initialize() got nil error, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Initialize() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
