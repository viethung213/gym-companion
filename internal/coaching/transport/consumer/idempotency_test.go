package consumer

import (
	"context"
	"errors"
	"testing"
)

// A handler failure must release the event: before the fix the claim stayed in
// outbox_log, every retry saw a duplicate and the event was dropped.
func TestHandleOnce_FailureIsRetried(t *testing.T) {
	stub := &stubOutbox{}
	env := &cloudEventEnvelope{ID: "evt-1", Type: "t"}
	calls := 0
	handle := func() error {
		calls++
		if calls == 1 {
			return errors.New("transient db error")
		}
		return nil
	}

	if err := handleOnce(context.Background(), stub, env, nil, handle); err == nil {
		t.Fatal("first delivery: want the handler error, got nil")
	}
	if err := handleOnce(context.Background(), stub, env, nil, handle); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if calls != 2 {
		t.Fatalf("handler ran %d times, want 2 (failure then retry)", calls)
	}

	// Once it succeeded, a redelivery is a duplicate.
	if err := handleOnce(context.Background(), stub, env, nil, handle); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if calls != 2 {
		t.Fatalf("handler ran %d times after success, want 2", calls)
	}
}
