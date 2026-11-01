package admin

import (
	"context"
	"testing"
	"time"
)

func TestAdminServer_Lifecycle(t *testing.T) {
	mock := &mockInspector{}
	server := NewServer("0", "admin-secret", mock)

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start()
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	err := <-errChan
	if err != nil {
		t.Fatalf("Server Start returned unexpected error: %v", err)
	}
}
