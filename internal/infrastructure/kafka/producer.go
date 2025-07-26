package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/IBM/sarama"
	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/metrics"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

type Producer struct {
	producer       sarama.SyncProducer
	config         *config.KafkaConfig
	circuitBreaker *CircuitBreaker
	metrics        *metrics.Metrics
}

// NewProducerForTesting creates a producer for testing purposes
func NewProducerForTesting(cfg *config.KafkaConfig) *Producer {
	return &Producer{
		config: cfg,
	}
}

// NewProducer creates a new Kafka producer
func NewProducer(cfg *config.KafkaConfig, metrics *metrics.Metrics) (*Producer, error) {
	var circuitBreaker *CircuitBreaker
	if cfg.CircuitBreakerEnabled {
		circuitBreaker = NewCircuitBreaker(
			cfg.FailureThreshold,
			cfg.SuccessThreshold,
			cfg.TimeoutDuration,
			cfg.ResetTimeout,
			metrics,
		)
		log.Printf("Circuit Breaker enabled with failure_threshold=%d, success_threshold=%d, timeout=%s, reset_timeout=%s",
			cfg.FailureThreshold, cfg.SuccessThreshold, cfg.TimeoutDuration, cfg.ResetTimeout)
	}

	if !cfg.IsKafkaEnabled() {
		log.Println("Kafka not enabled, creating producer in simulation mode")
		return &Producer{
			config:         cfg,
			circuitBreaker: circuitBreaker,
			metrics:        metrics,
		}, nil
	}

	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.RequiredAcks(cfg.RequiredAcks)
	config.Producer.Timeout = cfg.Timeout
	config.Producer.Retry.Max = cfg.MaxRetries
	config.Producer.Retry.Backoff = cfg.RetryDelay
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	config.Producer.Compression = sarama.CompressionSnappy

	producer, err := sarama.NewSyncProducer(cfg.GetKafkaBrokers(), config)
	if err != nil {
		log.Printf("Failed to create Kafka producer: %v", err)
		log.Println("Creating producer in simulation mode")
		return &Producer{
			config:         cfg,
			circuitBreaker: circuitBreaker,
			metrics:        metrics,
		}, nil
	}

	log.Printf("Kafka producer created successfully for brokers: %v", cfg.GetKafkaBrokers())

	return &Producer{
		producer:       producer,
		config:         cfg,
		circuitBreaker: circuitBreaker,
		metrics:        metrics,
	}, nil
}

// PublishEvent publishes an outbox event to Kafka with circuit breaker protection
func (p *Producer) PublishEvent(ctx context.Context, event *models.OutboxEvent) error {
	if p.circuitBreaker != nil {
		return p.publishEventWithCircuitBreaker(ctx, event)
	}

	return p.publishEventDirectly(ctx, event)
}

// publishEventWithCircuitBreaker publishes an event using circuit breaker protection
func (p *Producer) publishEventWithCircuitBreaker(ctx context.Context, event *models.OutboxEvent) error {
	return p.circuitBreaker.Execute(ctx, func() error {
		return p.publishEventDirectly(ctx, event)
	})
}

// publishEventDirectly publishes an event directly to Kafka with exponential retry
func (p *Producer) publishEventDirectly(ctx context.Context, event *models.OutboxEvent) error {
	if p.producer == nil {
		log.Println("Kafka producer not initialized, skipping event publication")
		return nil
	}

	message := &sarama.ProducerMessage{
		Topic: p.config.TopicEvents,
		Key:   sarama.StringEncoder(event.AggregateID),
		Value: sarama.StringEncoder(p.createEventPayload(event)),
		Headers: []sarama.RecordHeader{
			{Key: []byte("event_type"), Value: []byte(event.EventType)},
			{Key: []byte("aggregate_type"), Value: []byte(event.AggregateType)},
			{Key: []byte("event_id"), Value: []byte(event.ID.String())},
		},
	}

	var lastErr error
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		partition, offset, err := p.producer.SendMessage(message)
		if err == nil {
			log.Printf("Event published successfully to Kafka - Topic: %s, Partition: %d, Offset: %d, EventID: %s",
				p.config.TopicEvents, partition, offset, event.ID.String())
			return nil
		}

		lastErr = err
		log.Printf("Failed to publish event to Kafka (attempt %d/%d): %v, EventID: %s",
			attempt+1, p.config.MaxRetries+1, err, event.ID.String())

		if attempt == p.config.MaxRetries {
			break
		}

		delay := p.CalculateRetryDelay(attempt)

		if p.metrics != nil {
			p.metrics.RecordRetryDelayDuration(fmt.Sprintf("%d", attempt+1), delay)
		}

		log.Printf("Retrying in %v (attempt %d/%d)", delay, attempt+2, p.config.MaxRetries+1)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			continue
		}
	}

	return fmt.Errorf("failed to publish event after %d attempts: %w", p.config.MaxRetries+1, lastErr)
}

// CalculateRetryDelay calculates the delay for the next retry attempt
func (p *Producer) CalculateRetryDelay(attempt int) time.Duration {
	if !p.config.ExponentialRetryEnabled {
		return p.config.RetryDelay
	}

	delay := float64(p.config.BaseDelay)
	for i := 0; i < attempt; i++ {
		delay *= p.config.Multiplier
	}

	jitter := 0.5 + (rand.Float64() * 1.0)
	delay *= jitter

	if delay > float64(p.config.MaxDelay) {
		delay = float64(p.config.MaxDelay)
	}

	return time.Duration(delay)
}

// createEventPayload creates the event payload for Kafka
func (p *Producer) createEventPayload(event *models.OutboxEvent) string {
	payload := map[string]interface{}{
		"event_id":       event.ID.String(),
		"aggregate_id":   event.AggregateID,
		"aggregate_type": event.AggregateType,
		"event_type":     event.EventType,
		"event_data":     event.EventData,
		"created_at":     event.CreatedAt.Format(time.RFC3339),
	}

	if event.EventMetadata != nil {
		payload["event_metadata"] = event.EventMetadata
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal event payload: %v", err)
		return "{}"
	}

	return string(jsonPayload)
}

// Close closes the Kafka producer
func (p *Producer) Close() error {
	if p.producer != nil {
		if err := p.producer.Close(); err != nil {
			return fmt.Errorf("failed to close Kafka producer: %w", err)
		}
		log.Println("Kafka producer closed successfully")
	}
	return nil
}

// IsConnected returns true if the producer is connected
func (p *Producer) IsConnected() bool {
	return p.producer != nil
}

// GetConfig returns the Kafka configuration
func (p *Producer) GetConfig() *config.KafkaConfig {
	return p.config
}

// GetCircuitBreakerStats returns circuit breaker statistics if enabled
func (p *Producer) GetCircuitBreakerStats() map[string]interface{} {
	if p.circuitBreaker == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	stats := p.circuitBreaker.GetStats()
	stats["enabled"] = true
	return stats
}

// IsCircuitBreakerOpen returns true if the circuit breaker is open
func (p *Producer) IsCircuitBreakerOpen() bool {
	if p.circuitBreaker == nil {
		return false
	}
	return p.circuitBreaker.IsOpen()
}

// IsCircuitBreakerHalfOpen returns true if the circuit breaker is half-open
func (p *Producer) IsCircuitBreakerHalfOpen() bool {
	if p.circuitBreaker == nil {
		return false
	}
	return p.circuitBreaker.IsHalfOpen()
}

// ForceCircuitBreakerOpen forces the circuit breaker to open state
func (p *Producer) ForceCircuitBreakerOpen() {
	if p.circuitBreaker != nil {
		p.circuitBreaker.ForceOpen()
	}
}

// ForceCircuitBreakerClose forces the circuit breaker to closed state
func (p *Producer) ForceCircuitBreakerClose() {
	if p.circuitBreaker != nil {
		p.circuitBreaker.ForceClose()
	}
}

// IsExponentialRetryEnabled returns whether exponential retry is enabled
func (p *Producer) IsExponentialRetryEnabled() bool {
	return p.config.ExponentialRetryEnabled
}

// GetRetryConfig returns the current retry configuration
func (p *Producer) GetRetryConfig() map[string]interface{} {
	return map[string]interface{}{
		"exponential_retry_enabled": p.config.ExponentialRetryEnabled,
		"max_retries":               p.config.MaxRetries,
		"base_delay":                p.config.BaseDelay.String(),
		"max_delay":                 p.config.MaxDelay.String(),
		"multiplier":                p.config.Multiplier,
		"retry_delay":               p.config.RetryDelay.String(),
	}
}
