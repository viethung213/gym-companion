package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/transport/consumer"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

func TestConsumerManager_NilRegistry(t *testing.T) {
	mgr := consumer.NewConsumerManager(nil, nil, nil, nil, nil, nil)
	if mgr != nil {
		t.Fatal("expected nil ConsumerManager when registry is nil")
	}

	// Methods on nil receiver must be safe and not panic
	mgr.Start(context.Background())
	mgr.Wait()

	if mgr.PrimaryConsumer() != nil {
		t.Fatal("expected nil PrimaryConsumer for nil manager")
	}
}

func TestConsumerManager_WithRegistry(t *testing.T) {
	registry := sharedKafka.GetRegistry()
	mgr := consumer.NewConsumerManager(registry, nil, nil, nil, nil, nil)
	if mgr == nil {
		t.Fatal("expected non-nil ConsumerManager when registry is provided")
	}

	primary := mgr.PrimaryConsumer()
	if primary == nil {
		t.Fatal("expected non-nil PrimaryConsumer fallback")
	}

	ctx, cancel := context.WithCancel(context.Background())
	mgr.Start(ctx)

	// Cancel context to stop any started consumers
	cancel()

	done := make(chan struct{})
	go func() {
		mgr.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Succeeded gracefully
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ConsumerManager to stop")
	}
}
