package kafka

import (
	"fmt"
	"sync"

	"github.com/segmentio/kafka-go"
)

// Registry manages and isolates database connection pools and kafka readers/writers for all modules.
type Registry struct {
	mu      sync.RWMutex
	writers map[string]*kafka.Writer
	readers map[string]*kafka.Reader
}

var (
	//nolint:gochecknoglobals // Singleton database registry instance.
	instance *Registry
	//nolint:gochecknoglobals // Ensures singleton initialization once.
	once sync.Once
)

// GetRegistry returns the singleton instance of the connection Registry.
func GetRegistry() *Registry {
	once.Do(func() {
		instance = &Registry{
			writers: make(map[string]*kafka.Writer),
			readers: make(map[string]*kafka.Reader),
		}
	})
	return instance
}

// GetWriter retrieves or instantiates a kafka.Writer dedicated to a specific module.
func (r *Registry) GetWriter(module string, brokers []string) (*kafka.Writer, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no brokers provided for module %s", module)
	}

	r.mu.RLock()
	w, exists := r.writers[module]
	r.mu.RUnlock()

	if exists {
		return w, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	w, exists = r.writers[module]
	if exists {
		return w, nil
	}

	w = &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.Hash{},
		AllowAutoTopicCreation: true,
		RequiredAcks:           kafka.RequireAll,
	}

	r.writers[module] = w
	return w, nil
}

// GetReader retrieves or instantiates a kafka.Reader dedicated to a consumer group and topic.
func (r *Registry) GetReader(consumerGroup, topic string, brokers []string) (*kafka.Reader, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no brokers provided for topic %s", topic)
	}

	key := fmt.Sprintf("%s:%s", consumerGroup, topic)
	r.mu.RLock()
	reader, exists := r.readers[key]
	r.mu.RUnlock()

	if exists {
		return reader, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	reader, exists = r.readers[key]
	if exists {
		return reader, nil
	}

	reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     consumerGroup,
		Topic:       topic,
		MinBytes:    10,
		MaxBytes:    10 * 1024 * 1024,
		StartOffset: kafka.FirstOffset,
	})

	r.readers[key] = reader
	return reader, nil
}

// CloseAll closes all open Kafka writers and readers in the registry.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for module, w := range r.writers {
		if w != nil {
			_ = w.Close()
		}
		delete(r.writers, module)
	}

	for key, reader := range r.readers {
		if reader != nil {
			_ = reader.Close()
		}
		delete(r.readers, key)
	}
}
