package kafka

import (
	"fmt"
	"sync"

	"github.com/segmentio/kafka-go"
)

// Registry manages and isolates database connection pools for all modules.
type Registry struct {
	mu      sync.RWMutex
	writers map[string]*kafka.Writer
}

var (
	//nolint:gochecknoglobals // Singleton database registry instance.
	instance *Registry
	//nolint:gochecknoglobals // Ensures singleton initialization once.
	once sync.Once
)

// GetRegistry returns the singleton instance of the database connection Registry.
func GetRegistry() *Registry {
	once.Do(func() {
		instance = &Registry{
			writers: make(map[string]*kafka.Writer),
		}
	})
	return instance
}

// GetWriter retrieves or instantiates a kafka.Writer dedicated to a specific module.
func (r *Registry) GetWriter(module string, brokers []string) (*kafka.Writer, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no brokers provided for module %s", module)
	}

	// Thread-safe read lock check
	r.mu.RLock()
	w, exists := r.writers[module]
	r.mu.RUnlock()

	if exists {
		return w, nil
	}

	// Lock for writing and double-check
	r.mu.Lock()
	defer r.mu.Unlock()

	w, exists = r.writers[module]
	if exists {
		return w, nil
	}

	// Instantiate writer for this module
	w = &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.Hash{},
		AllowAutoTopicCreation: true,
		RequiredAcks:           kafka.RequireAll, // Ensures robust delivery (ACID-like safety)
	}

	r.writers[module] = w
	return w, nil
}

// CloseAll closes all open Kafka writers in the registry.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for module, w := range r.writers {
		if w != nil {
			_ = w.Close()
		}
		delete(r.writers, module)
	}
}
