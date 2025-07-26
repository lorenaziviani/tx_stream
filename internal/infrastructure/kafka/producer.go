package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

type Producer struct {
	producer       sarama.SyncProducer
	config         *config.KafkaConfig
	circuitBreaker *CircuitBreaker
}

func NewProducer(cfg *config.KafkaConfig) (*Producer, error) {
	var circuitBreaker *CircuitBreaker
	if cfg.CircuitBreakerEnabled {
		circuitBreaker = NewCircuitBreaker(
			cfg.FailureThreshold,
			cfg.SuccessThreshold,
			cfg.TimeoutDuration,
			cfg.ResetTimeout,
		)
		log.Printf("Circuit Breaker enabled with failure_threshold=%d, success_threshold=%d, timeout=%s, reset_timeout=%s",
			cfg.FailureThreshold, cfg.SuccessThreshold, cfg.TimeoutDuration, cfg.ResetTimeout)
	}

	if !cfg.IsKafkaEnabled() {
		log.Println("Kafka not enabled, creating producer in simulation mode")
		return &Producer{
			config:         cfg,
			circuitBreaker: circuitBreaker,
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
		}, nil
	}

	log.Printf("Kafka producer created successfully for brokers: %v", cfg.GetKafkaBrokers())

	return &Producer{
		producer:       producer,
		config:         cfg,
		circuitBreaker: circuitBreaker,
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

// publishEventDirectly publishes an event directly to Kafka
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
			{Key: []byte("timestamp"), Value: []byte(event.CreatedAt.Format(time.RFC3339))},
		},
	}

	partition, offset, err := p.producer.SendMessage(message)
	if err != nil {
		return fmt.Errorf("failed to publish event %s: %w", event.ID, err)
	}

	log.Printf("Event %s published successfully to topic %s (partition: %d, offset: %d)",
		event.ID, p.config.TopicEvents, partition, offset)

	return nil
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
