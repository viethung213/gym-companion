package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	segmentkafka "github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/shared/cloudevent"
)

type fakeReader struct {
	committed   []segmentkafka.Message
	commitErr   error
	commitCalls int
}

func (r *fakeReader) FetchMessage(context.Context) (segmentkafka.Message, error) {
	return segmentkafka.Message{}, errors.New("not implemented")
}

func (r *fakeReader) CommitMessages(
	_ context.Context,
	messages ...segmentkafka.Message,
) error {
	r.commitCalls++
	if r.commitErr != nil {
		return r.commitErr
	}
	r.committed = append(r.committed, messages...)
	return nil
}

func TestCommitFailureDoesNotAdvance(t *testing.T) {
	t.Parallel()

	reader := &fakeReader{commitErr: errors.New("coordinator unavailable")}
	consumer := newConsumer(reader, nil, nil, &fakeDeadLetters{})
	message := &segmentkafka.Message{Offset: 10}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := consumer.commit(ctx, message); !errors.Is(err, context.Canceled) {
		t.Fatalf("error got = %v, want context canceled", err)
	}
	if len(reader.committed) != 0 || reader.commitCalls != 1 {
		t.Errorf(
			"committed = %d calls = %d, want no commit and one attempt",
			len(reader.committed),
			reader.commitCalls,
		)
	}
}

func (r *fakeReader) Close() error { return nil }

type fakeDeadLetters struct {
	err       error
	publishes int
	attention int
}

func (p *fakeDeadLetters) Publish(
	_ context.Context,
	_ *segmentkafka.Message,
	_ string,
	attempts int,
) error {
	p.publishes++
	p.attention = attempts
	return p.err
}

func TestHandleFetchedMessageCommitsAfterDLQ(t *testing.T) {
	t.Parallel()

	reader := &fakeReader{}
	deadLetters := &fakeDeadLetters{}
	consumer := newConsumer(reader, nil, nil, deadLetters)
	message := &segmentkafka.Message{Offset: 10, Value: []byte(`not-json`)}

	if err := consumer.handleFetchedMessage(context.Background(), message); err != nil {
		t.Fatalf("handle message: %v", err)
	}
	if got := len(reader.committed); got != 1 {
		t.Errorf("committed messages got = %d, want = 1", got)
	}
	if deadLetters.publishes != 1 || deadLetters.attention != 1 {
		t.Errorf(
			"DLQ calls got = %d attempts = %d, want one call with one attempt",
			deadLetters.publishes,
			deadLetters.attention,
		)
	}
}

func TestHandleFetchedMessageDoesNotCommitWhenDLQFails(t *testing.T) {
	t.Parallel()

	reader := &fakeReader{}
	deadLetters := &fakeDeadLetters{err: errors.New("broker unavailable")}
	consumer := newConsumer(reader, nil, nil, deadLetters)
	message := &segmentkafka.Message{Offset: 10, Value: []byte(`not-json`)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := consumer.handleFetchedMessage(ctx, message); !errors.Is(err, context.Canceled) {
		t.Fatalf("error got = %v, want context canceled", err)
	}
	if got := len(reader.committed); got != 0 {
		t.Errorf("committed messages got = %d, want = 0", got)
	}
}

func TestDecodeProfileCompletedUsesProtobufJSON(t *testing.T) {
	t.Parallel()

	encoded, err := cloudevent.Encode(
		"profile-event-1",
		"services/profile-service",
		profileCompletedEventType,
		time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC),
		[]byte(`{
			"userId":"user-1",
			"goals":["strength"],
			"registeredInjuries":["knee"],
			"experienceLevel":"BEGINNER"
		}`),
	)
	if err != nil {
		t.Fatalf("encode CloudEvent: %v", err)
	}

	_, got, err := decodeProfileCompleted(&segmentkafka.Message{Value: encoded})
	if err != nil {
		t.Fatalf("decode ProfileCompleted: %v", err)
	}
	if got.UserID != "user-1" || got.ExperienceLevel != "beginner" {
		t.Errorf("event got = %#v", got)
	}
}

func TestDecodeProfileCompletedRejectsUnknownExperience(t *testing.T) {
	t.Parallel()

	encoded, err := cloudevent.Encode(
		"profile-event-1",
		"services/profile-service",
		profileCompletedEventType,
		time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC),
		[]byte(`{"userId":"user-1","experienceLevel":"EXPERT"}`),
	)
	if err != nil {
		t.Fatalf("encode CloudEvent: %v", err)
	}

	_, _, err = decodeProfileCompleted(&segmentkafka.Message{Value: encoded})
	if !errors.Is(err, errPermanentMessage) {
		t.Fatalf("error got = %v, want permanent message error", err)
	}
}

func TestDecodeProfileCompletedRejectsUntrustedSource(t *testing.T) {
	t.Parallel()

	encoded, err := cloudevent.Encode(
		"profile-event-1",
		"services/untrusted-producer",
		profileCompletedEventType,
		time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC),
		[]byte(`{"userId":"user-1","experienceLevel":"BEGINNER"}`),
	)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	_, _, err = decodeProfileCompleted(&segmentkafka.Message{Value: encoded})
	if !errors.Is(err, errPermanentMessage) {
		t.Fatalf("error got = %v, want permanent message error", err)
	}
}
