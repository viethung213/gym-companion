package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
)

// OutboxPublisherWorker periodically polls pending events from database outbox and pushes them to Kafka.
type OutboxPublisherWorker struct {
	outboxRepo port.OutboxRepository
	writer     *kafka.Writer
	interval   time.Duration
	stopCh     chan struct{}
}

// NewOutboxPublisherWorker constructs a new OutboxPublisherWorker instance.
func NewOutboxPublisherWorker(outboxRepo port.OutboxRepository, writer *kafka.Writer, interval time.Duration) *OutboxPublisherWorker {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &OutboxPublisherWorker{
		outboxRepo: outboxRepo,
		writer:     writer,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the periodic polling sweep loop until the context or worker is stopped.
func (p *OutboxPublisherWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	log.Printf("[SoloMatch] Starting Outbox Publisher with SKIP LOCKED (polling: %v)...", p.interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[SoloMatch] PANIC recovered in Outbox Publisher tick: %v", r)
					}
				}()
				if err := p.processPendingEvents(ctx); err != nil {
					log.Printf("[SoloMatch] Outbox Publisher process error: %v", err)
				}
			}()
		}
	}
}

// Stop signals the worker to terminate gracefully.
func (p *OutboxPublisherWorker) Stop() {
	close(p.stopCh)
}

func (p *OutboxPublisherWorker) processPendingEvents(ctx context.Context) error {
	// ClaimBatch sử dụng SELECT ... FOR UPDATE SKIP LOCKED
	// Khóa an toàn 50 sự kiện trong 10 giây, cho phép các instance khác bỏ qua các dòng này để xử lý song song
	records, err := p.outboxRepo.ClaimBatch(ctx, 50, 10*time.Second)
	if err != nil {
		return fmt.Errorf("claim batch outbox events: %w", err)
	}

	if len(records) == 0 {
		return nil
	}

	publishedIDs := make([]string, 0, len(records))

	for _, rec := range records {
		if p.writer != nil {
			err = p.writer.WriteMessages(ctx, kafka.Message{
				Key:   []byte(rec.PartitionKey),
				Value: rec.Payload,
				Time:  time.Now().UTC(),
			})
			if err != nil {
				log.Printf("[SoloMatch] Write Kafka message error for outbox event %s: %v", rec.EventID, err)
				return fmt.Errorf("write kafka message for event %s: %w", rec.EventID, err)
			}
		}
		publishedIDs = append(publishedIDs, rec.ID)
	}

	// Đánh dấu toàn bộ batch đã published thành công
	if err := p.outboxRepo.MarkAsPublished(ctx, publishedIDs); err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	return nil
}
