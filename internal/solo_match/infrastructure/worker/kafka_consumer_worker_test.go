package worker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/solo_match/infrastructure/worker"
)

type mockKafkaReader struct {
	mu       sync.Mutex
	messages []kafka.Message
	closed   bool
	readErr  error
}

func (m *mockKafkaReader) ReadMessage(ctx context.Context) (kafka.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.readErr != nil {
		return kafka.Message{}, m.readErr
	}

	if len(m.messages) > 0 {
		msg := m.messages[0]
		m.messages = m.messages[1:]
		return msg, nil
	}

	// Block until context canceled
	m.mu.Unlock()
	<-ctx.Done()
	m.mu.Lock()
	return kafka.Message{}, ctx.Err()
}

func (m *mockKafkaReader) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

type mockMessageHandler struct {
	mu       sync.Mutex
	payloads [][]byte
	panicOn  bool
}

func (h *mockMessageHandler) HandleMessage(_ context.Context, rawPayload []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.panicOn {
		panic("test panic")
	}
	h.payloads = append(h.payloads, rawPayload)
	return nil
}

func TestKafkaConsumerWorker(t *testing.T) {
	t.Run("returns early if reader is nil", func(t *testing.T) {
		w := worker.NewKafkaConsumerWorker("NilReaderWorker", nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		w.Start(ctx) // Should not panic or block
	})

	t.Run("successfully reads message and dispatches to handlers", func(t *testing.T) {
		reader := &mockKafkaReader{
			messages: []kafka.Message{
				{Value: []byte("test_payload_1")},
				{Value: []byte("test_payload_2")},
			},
		}

		handler1 := &mockMessageHandler{}
		handler2 := &mockMessageHandler{}

		w := worker.NewKafkaConsumerWorker("TestWorker", reader, handler1, handler2)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			w.Start(ctx)
			close(done)
		}()

		// Wait briefly for messages to be consumed
		time.Sleep(50 * time.Millisecond)
		cancel()
		<-done

		handler1.mu.Lock()
		defer handler1.mu.Unlock()
		if len(handler1.payloads) != 2 {
			t.Errorf("expected handler1 to receive 2 messages, got %d", len(handler1.payloads))
		}

		handler2.mu.Lock()
		defer handler2.mu.Unlock()
		if len(handler2.payloads) != 2 {
			t.Errorf("expected handler2 to receive 2 messages, got %d", len(handler2.payloads))
		}

		reader.mu.Lock()
		defer reader.mu.Unlock()
		if !reader.closed {
			t.Errorf("expected reader to be closed on worker exit")
		}
	})

	t.Run("recovers gracefully from handler panic without crashing worker", func(t *testing.T) {
		reader := &mockKafkaReader{
			messages: []kafka.Message{
				{Value: []byte("payload_causing_panic")},
			},
		}

		panicHandler := &mockMessageHandler{panicOn: true}
		w := worker.NewKafkaConsumerWorker("PanicTestWorker", reader, panicHandler)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			w.Start(ctx)
			close(done)
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()
		<-done

		reader.mu.Lock()
		defer reader.mu.Unlock()
		if !reader.closed {
			t.Errorf("expected reader to be closed after panic recovery")
		}
	})

	t.Run("exits cleanly on context canceled read error", func(t *testing.T) {
		reader := &mockKafkaReader{
			readErr: context.Canceled,
		}

		w := worker.NewKafkaConsumerWorker("ContextCanceledWorker", reader)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		done := make(chan struct{})
		go func() {
			w.Start(ctx)
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(500 * time.Millisecond):
			t.Fatal("worker did not exit cleanly on context cancel")
		}
	})
}
