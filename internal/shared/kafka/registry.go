package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// Registry manages and isolates database connection pools and kafka readers/writers for all modules.
type Registry struct {
	mu      sync.RWMutex
	brokers []string
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
		brokersStr := getFirstEnv("KAFKA_BROKERS", "KAFKA_BOOTSTRAP_SERVERS")
		if brokersStr == "" {
			brokersStr = "localhost:9092"
		}
		var brokers []string
		for _, b := range strings.Split(brokersStr, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				brokers = append(brokers, b)
			}
		}
		instance = &Registry{
			brokers: brokers,
			writers: make(map[string]*kafka.Writer),
			readers: make(map[string]*kafka.Reader),
		}
	})
	return instance
}

// GetBrokers returns a copy of the default configured brokers.
func (r *Registry) GetBrokers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]string, len(r.brokers))
	copy(res, r.brokers)
	return res
}

func (r *Registry) resolveBrokers(brokers ...[]string) ([]string, error) {
	if len(brokers) > 0 && len(brokers[0]) > 0 {
		return brokers[0], nil
	}
	r.mu.RLock()
	defaultBrokers := r.brokers
	r.mu.RUnlock()
	if len(defaultBrokers) > 0 {
		return defaultBrokers, nil
	}
	return nil, errors.New("no kafka brokers configured")
}

// GetWriter retrieves or instantiates a kafka.Writer dedicated to a specific module.
func (r *Registry) GetWriter(module string, brokers ...[]string) (*kafka.Writer, error) {
	targetBrokers, err := r.resolveBrokers(brokers...)
	if err != nil {
		return nil, fmt.Errorf("resolve brokers for module %s: %w", module, err)
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

	_, transport, err := buildSecurityConfig()
	if err != nil {
		return nil, fmt.Errorf("build security config for writer (%s): %w", module, err)
	}

	ensureTopicExists(targetBrokers, module)

	w = &kafka.Writer{
		Addr:                   kafka.TCP(targetBrokers...),
		Topic:                  module,
		Balancer:               &kafka.Hash{},
		AllowAutoTopicCreation: true,
		RequiredAcks:           kafka.RequireAll,
	}
	if transport != nil {
		w.Transport = transport
	}

	r.writers[module] = w
	return w, nil
}

// GetDLQWriter returns a kafka.Writer pointed to the system-wide Dead Letter Queue topic (SystemDLQTopic).
func (r *Registry) GetDLQWriter(brokers ...[]string) (*kafka.Writer, error) {
	return r.GetWriter(SystemDLQTopic, brokers...)
}

// GetReader retrieves or instantiates a kafka.Reader dedicated to a consumer group and topic.
func (r *Registry) GetReader(consumerGroup, topic string, brokers ...[]string) (*kafka.Reader, error) {
	return r.GetReaderInstance(consumerGroup, topic, "", brokers...)
}

// GetReaderInstance retrieves or instantiates an independent kafka.Reader for a consumer group, topic, and instance identifier.
// Different instanceIDs yield separate kafka.Reader instances within the same consumer group, enabling multi-partition consumption.
func (r *Registry) GetReaderInstance(consumerGroup, topic, instanceID string, brokers ...[]string) (*kafka.Reader, error) {
	targetBrokers, err := r.resolveBrokers(brokers...)
	if err != nil {
		return nil, fmt.Errorf("resolve brokers for topic %s: %w", topic, err)
	}

	key := fmt.Sprintf("%s:%s", consumerGroup, topic)
	if instanceID != "" {
		key = fmt.Sprintf("%s:%s:%s", consumerGroup, topic, instanceID)
	}
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

	numPartitions := 1
	if topic == "notification.events.high-priority" {
		numPartitions = 3
	}
	ensureTopicExistsWithPartitions(targetBrokers, topic, numPartitions)

	dialer, _, err := buildSecurityConfig()
	if err != nil {
		return nil, fmt.Errorf("build security config for reader (%s/%s): %w", consumerGroup, topic, err)
	}

	readerConfig := kafka.ReaderConfig{
		Brokers:     targetBrokers,
		GroupID:     consumerGroup,
		Topic:       topic,
		MinBytes:    10,
		MaxBytes:    10 * 1024 * 1024,
		StartOffset: kafka.FirstOffset,
	}
	if dialer != nil {
		readerConfig.Dialer = dialer
	}

	reader = kafka.NewReader(readerConfig)
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

func buildSecurityConfig() (*kafka.Dialer, *kafka.Transport, error) {
	protocol := strings.ToUpper(getEnvOrDefault("KAFKA_SECURITY_PROTOCOL", "PLAINTEXT"))
	if protocol == "" || protocol == "PLAINTEXT" {
		return nil, nil, nil
	}

	var saslMech sasl.Mechanism
	if strings.HasPrefix(protocol, "SASL") {
		user := getFirstEnv("KAFKA_USER", "KAFKA_SASL_USER", "KAFKA_SASL_USERNAME")
		pass := getFirstEnv("KAFKA_PASSWORD", "KAFKA_SASL_PASSWORD")
		mechanism := strings.ToUpper(getEnvOrDefault("KAFKA_SASL_MECHANISM", "SCRAM-SHA-256"))

		var err error
		switch mechanism {
		case "SCRAM-SHA-256":
			saslMech, err = scram.Mechanism(scram.SHA256, user, pass)
			if err != nil {
				return nil, nil, fmt.Errorf("create SCRAM-SHA-256 mechanism: %w", err)
			}
		case "SCRAM-SHA-512":
			saslMech, err = scram.Mechanism(scram.SHA512, user, pass)
			if err != nil {
				return nil, nil, fmt.Errorf("create SCRAM-SHA-512 mechanism: %w", err)
			}
		case "PLAIN":
			saslMech = plain.Mechanism{
				Username: user,
				Password: pass,
			}
		default:
			return nil, nil, fmt.Errorf("unsupported SASL mechanism: %s", mechanism)
		}
	}

	var tlsConfig *tls.Config
	if strings.Contains(protocol, "SSL") || strings.Contains(protocol, "TLS") {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		caPath := getFirstEnv("KAFKA_CA_CERT_PATH", "KAFKA_CA_PATH", "KAFKA_CA_CERT")
		if caPath != "" {
			caCert, err := os.ReadFile(caPath)
			if err != nil {
				return nil, nil, fmt.Errorf("read kafka ca cert from %s: %w", caPath, err)
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caCert) {
				return nil, nil, fmt.Errorf("failed to parse kafka ca cert from %s", caPath)
			}
			tlsConfig.RootCAs = caPool
		} else if os.Getenv("KAFKA_INSECURE_SKIP_VERIFY") == "true" {
			tlsConfig.InsecureSkipVerify = true
		}
	}

	clientID := getFirstEnv("KAFKA_CLIENT_ID", "KAFKA_CLIENT_NAME")

	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           tlsConfig,
		SASLMechanism: saslMech,
		ClientID:      clientID,
	}

	transport := &kafka.Transport{
		TLS:      tlsConfig,
		SASL:     saslMech,
		ClientID: clientID,
	}

	return dialer, transport, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getFirstEnv(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func ensureTopicExists(brokers []string, topic string) {
	ensureTopicExistsWithPartitions(brokers, topic, 1)
}

func ensureTopicExistsWithPartitions(brokers []string, topic string, numPartitions int) {
	if len(brokers) == 0 || topic == "" {
		return
	}
	if numPartitions <= 0 {
		numPartitions = 1
	}
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: 1,
	})
}
