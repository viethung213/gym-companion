package consumer_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/solo_match/transport/consumer"
)

type mockKafkaReader struct {
	mu       sync.Mutex
	messages []kafka.Message
	closed   bool
	readErr  error
}

func (m *mockKafkaReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
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

func (m *mockKafkaReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	return nil
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
		panic("test panic inside handler")
	}
	h.payloads = append(h.payloads, rawPayload)
	return nil
}

func TestTopicConsumer(t *testing.T) {
	t.Run("Nil reader terminates cleanly", func(t *testing.T) {
		w := consumer.NewTopicConsumer("NilReaderWorker", nil)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		w.Start(ctx)
	})

	t.Run("Processes messages and invokes handlers", func(t *testing.T) {
		reader := &mockKafkaReader{
			messages: []kafka.Message{
				{Value: []byte(`{"id":"1","type":"test.event"}`)},
				{Value: []byte(`{"id":"2","type":"test.event"}`)},
			},
		}
		handler1 := &mockMessageHandler{}
		handler2 := &mockMessageHandler{}

		w := consumer.NewTopicConsumer("TestWorker", reader, handler1, handler2)
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		w.Start(ctx)

		handler1.mu.Lock()
		count1 := len(handler1.payloads)
		handler1.mu.Unlock()

		handler2.mu.Lock()
		count2 := len(handler2.payloads)
		handler2.mu.Unlock()

		if count1 != 2 {
			t.Errorf("expected handler1 to receive 2 messages, got %d", count1)
		}
		if count2 != 2 {
			t.Errorf("expected handler2 to receive 2 messages, got %d", count2)
		}

		reader.mu.Lock()
		closed := reader.closed
		reader.mu.Unlock()
		if !closed {
			t.Errorf("expected reader to be closed after worker terminates")
		}
	})

	t.Run("Recovers from handler panic without crashing", func(t *testing.T) {
		reader := &mockKafkaReader{
			messages: []kafka.Message{
				{Value: []byte(`{"id":"panic","type":"test.event"}`)},
			},
		}
		panicHandler := &mockMessageHandler{panicOn: true}
		w := consumer.NewTopicConsumer("PanicTestWorker", reader, panicHandler)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("worker failed to recover from handler panic: %v", r)
			}
		}()

		w.Start(ctx)
	})

	t.Run("Graceful exit on context canceled", func(t *testing.T) {
		reader := &mockKafkaReader{}
		w := consumer.NewTopicConsumer("ContextCanceledWorker", reader)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		w.Start(ctx)

		reader.mu.Lock()
		closed := reader.closed
		reader.mu.Unlock()
		if !closed {
			t.Errorf("expected reader to be closed on context cancellation")
		}
	})
}
