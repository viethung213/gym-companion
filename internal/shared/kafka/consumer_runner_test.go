package kafka_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	sharedKafka "github.com/viethung213/gym-companion/internal/shared/kafka"
)

type mockMessageReader struct {
	mu           sync.Mutex
	messages     []kafka.Message
	fetchIndex   int
	fetchErr     error
	committed    []kafka.Message
	fetchCalls   int
	commitCalls  int
	cancelOnDone context.CancelFunc
}

func (m *mockMessageReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fetchCalls++

	if m.fetchErr != nil {
		return kafka.Message{}, m.fetchErr
	}

	if m.fetchIndex < len(m.messages) {
		msg := m.messages[m.fetchIndex]
		m.fetchIndex++
		return msg, nil
	}

	if m.cancelOnDone != nil {
		m.cancelOnDone()
	}
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (m *mockMessageReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commitCalls++
	m.committed = append(m.committed, msgs...)
	return nil
}

type mockMessageWriter struct {
	mu         sync.Mutex
	written    []kafka.Message
	writeErr   error
	writeCalls int
}

func (m *mockMessageWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++
	if m.writeErr != nil {
		return m.writeErr
	}
	m.written = append(m.written, msgs...)
	return nil
}

func TestRunConsumerLoop_SuccessFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := &mockMessageReader{
		messages: []kafka.Message{
			{Topic: "test.events", Offset: 1, Value: []byte("payload-1")},
		},
		cancelOnDone: cancel,
	}

	processCount := 0
	processFn := func(ctx context.Context, msg *kafka.Message) error {
		processCount++
		return nil
	}

	cfg := sharedKafka.ConsumerConfig{
		Name:        "test-consumer",
		MaxRetries:  3,
		BaseBackoff: 10 * time.Millisecond,
	}

	sharedKafka.RunConsumerLoop(ctx, reader, cfg, processFn)

	if processCount != 1 {
		t.Errorf("expected processCount = 1, got %d", processCount)
	}
	if len(reader.committed) != 1 {
		t.Errorf("expected 1 committed message, got %d", len(reader.committed))
	}
	if reader.committed[0].Offset != 1 {
		t.Errorf("expected committed offset = 1, got %d", reader.committed[0].Offset)
	}
}

func TestRunConsumerLoop_RetrySuccessOnThirdAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := &mockMessageReader{
		messages: []kafka.Message{
			{Topic: "test.events", Offset: 10, Value: []byte("payload-retry")},
		},
		cancelOnDone: cancel,
	}

	attempt := 0
	processFn := func(ctx context.Context, msg *kafka.Message) error {
		attempt++
		if attempt < 3 {
			return errors.New("transient error")
		}
		return nil
	}

	cfg := sharedKafka.ConsumerConfig{
		Name:        "test-retry-consumer",
		MaxRetries:  3,
		BaseBackoff: 5 * time.Millisecond,
	}

	sharedKafka.RunConsumerLoop(ctx, reader, cfg, processFn)

	if attempt != 3 {
		t.Errorf("expected attempt = 3, got %d", attempt)
	}
	if len(reader.committed) != 1 {
		t.Errorf("expected message to be committed after 3rd attempt, got %d", len(reader.committed))
	}
}

func TestRunConsumerLoop_ExhaustedRetries_RoutesToDLQAndCommits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := &mockMessageReader{
		messages: []kafka.Message{
			{Topic: "test.events", Offset: 42, Value: []byte("payload-fail")},
		},
		cancelOnDone: cancel,
	}

	dlqWriter := &mockMessageWriter{}
	onDLQCalled := false
	var onDLQErr error

	cfg := sharedKafka.ConsumerConfig{
		Name:        "test-dlq-consumer",
		MaxRetries:  3,
		BaseBackoff: 5 * time.Millisecond,
		DLQWriter:   dlqWriter,
		OnDLQ: func(ctx context.Context, msg *kafka.Message, lastErr error) error {
			onDLQCalled = true
			onDLQErr = lastErr
			return nil
		},
	}

	attempt := 0
	processFn := func(ctx context.Context, msg *kafka.Message) error {
		attempt++
		return errors.New("persistent error")
	}

	sharedKafka.RunConsumerLoop(ctx, reader, cfg, processFn)

	if attempt != 3 {
		t.Errorf("expected attempt = 3, got %d", attempt)
	}
	if !onDLQCalled {
		t.Errorf("expected OnDLQ callback to be called")
	}
	if onDLQErr == nil || onDLQErr.Error() != "persistent error" {
		t.Errorf("expected onDLQErr = persistent error, got %v", onDLQErr)
	}
	if len(dlqWriter.written) != 1 {
		t.Fatalf("expected 1 DLQ message written, got %d", len(dlqWriter.written))
	}

	// Verify DLQ headers
	dlqMsg := dlqWriter.written[0]
	headerMap := make(map[string]string)
	for _, h := range dlqMsg.Headers {
		headerMap[h.Key] = string(h.Value)
	}

	if headerMap["x-original-topic"] != "test.events" {
		t.Errorf("expected x-original-topic = test.events, got %s", headerMap["x-original-topic"])
	}
	if headerMap["x-retry-count"] != "3" {
		t.Errorf("expected x-retry-count = 3, got %s", headerMap["x-retry-count"])
	}
	if headerMap["x-error-message"] != "persistent error" {
		t.Errorf("expected x-error-message = persistent error, got %s", headerMap["x-error-message"])
	}

	// Verify CommitMessages was called
	if len(reader.committed) != 1 {
		t.Errorf("expected message to be committed after DLQ routing, got %d", len(reader.committed))
	}
}

func TestRunConsumerLoop_NilReader(t *testing.T) {
	ctx := context.Background()
	// Should not panic
	sharedKafka.RunConsumerLoop(ctx, nil, sharedKafka.ConsumerConfig{}, func(ctx context.Context, msg *kafka.Message) error {
		return nil
	})
}

func TestRunConsumerLoop_PanicRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader := &mockMessageReader{
		messages: []kafka.Message{
			{Topic: "panic.events", Offset: 1, Value: []byte("bad-payload")},
		},
		cancelOnDone: cancel,
	}

	panicCalls := 0
	processFn := func(ctx context.Context, msg *kafka.Message) error {
		panicCalls++
		panic("nil pointer dereference test")
	}

	onDLQCalled := false
	var onDLQErr error
	cfg := sharedKafka.ConsumerConfig{
		Name:        "panic-consumer",
		MaxRetries:  3,
		BaseBackoff: 5 * time.Millisecond,
		OnDLQ: func(ctx context.Context, msg *kafka.Message, lastErr error) error {
			onDLQCalled = true
			onDLQErr = lastErr
			return nil
		},
	}

	// Should not crash the process; should recover panic, retry 3 times, route to DLQ, and commit
	sharedKafka.RunConsumerLoop(ctx, reader, cfg, processFn)

	if panicCalls != 3 {
		t.Errorf("expected 3 panic attempts, got %d", panicCalls)
	}
	if !onDLQCalled {
		t.Errorf("expected OnDLQ to be called after panics exhausted")
	}
	if onDLQErr == nil {
		t.Errorf("expected onDLQErr to be non-nil")
	}
	if len(reader.committed) != 1 {
		t.Errorf("expected message to be committed after panic exhausted, got %d", len(reader.committed))
	}
}

func TestRunConsumerLoop_ExhaustedRetries_WithoutDLQ_DoesNotCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	reader := &mockMessageReader{
		messages: []kafka.Message{
			{Topic: "fail.events", Offset: 100, Value: []byte("fail-payload")},
		},
	}

	calls := 0
	processFn := func(ctx context.Context, msg *kafka.Message) error {
		calls++
		if calls >= 3 {
			cancel() // Cancel context once all retries are exhausted to exit loop
		}
		return errors.New("database connection refused")
	}

	cfg := sharedKafka.ConsumerConfig{
		Name:        "no-dlq-consumer",
		MaxRetries:  3,
		BaseBackoff: 5 * time.Millisecond,
		// No DLQWriter and No OnDLQ configured
	}

	sharedKafka.RunConsumerLoop(ctx, reader, cfg, processFn)

	if calls < 3 {
		t.Errorf("expected at least 3 attempts, got %d", calls)
	}
	if len(reader.committed) != 0 {
		t.Errorf("expected 0 committed messages when failure occurs without DLQ, got %d", len(reader.committed))
	}
}

func TestRunConsumerLoop_ExhaustedRetries_DLQWriteFailure_DoesNotCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	reader := &mockMessageReader{
		messages: []kafka.Message{
			{Topic: "dlq-fail.events", Offset: 101, Value: []byte("dlq-fail-payload")},
		},
	}

	dlqWriter := &mockMessageWriter{
		writeErr: errors.New("kafka broker unavailable for DLQ"),
	}

	calls := 0
	processFn := func(ctx context.Context, msg *kafka.Message) error {
		calls++
		if calls >= 3 {
			cancel() // Cancel context once DLQ write fails to exit loop
		}
		return errors.New("business error")
	}

	cfg := sharedKafka.ConsumerConfig{
		Name:        "failing-dlq-consumer",
		MaxRetries:  3,
		BaseBackoff: 5 * time.Millisecond,
		DLQWriter:   dlqWriter,
	}

	sharedKafka.RunConsumerLoop(ctx, reader, cfg, processFn)

	if calls < 3 {
		t.Errorf("expected at least 3 attempts, got %d", calls)
	}
	if len(reader.committed) != 0 {
		t.Errorf("expected 0 committed messages when DLQ publish fails, got %d", len(reader.committed))
	}
}
