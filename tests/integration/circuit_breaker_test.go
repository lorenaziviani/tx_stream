package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/google/uuid"
	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/kafka"
	"github.com/lorenaziviani/txstream/internal/infrastructure/metrics"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

func TestCircuitBreakerIntegration(t *testing.T) {
	t.Run("producer_with_circuit_breaker_enabled", func(t *testing.T) {
		kafkaConfig := &config.KafkaConfig{
			Brokers:               []string{"localhost:9092"},
			TopicEvents:           "test.events",
			RequiredAcks:          1,
			Timeout:               30 * time.Second,
			MaxRetries:            3,
			RetryDelay:            1 * time.Second,
			CircuitBreakerEnabled: true,
			FailureThreshold:      2,
			SuccessThreshold:      2,
			TimeoutDuration:       5 * time.Second,
			ResetTimeout:          10 * time.Second,
		}

		metrics := metrics.NewMetrics()
		producer, err := kafka.NewProducer(kafkaConfig, metrics)
		require.NoError(t, err)
		defer producer.Close()

		stats := producer.GetCircuitBreakerStats()
		assert.True(t, stats["enabled"].(bool))
		assert.Equal(t, "CLOSED", stats["state"])
		assert.Equal(t, 0, stats["failure_count"])
		assert.Equal(t, 0, stats["success_count"])
	})

	t.Run("producer_with_circuit_breaker_disabled", func(t *testing.T) {
		kafkaConfig := &config.KafkaConfig{
			Brokers:               []string{"localhost:9092"},
			TopicEvents:           "test.events",
			RequiredAcks:          1,
			Timeout:               30 * time.Second,
			MaxRetries:            3,
			RetryDelay:            1 * time.Second,
			CircuitBreakerEnabled: false,
		}

		metrics := metrics.NewMetrics()
		producer, err := kafka.NewProducer(kafkaConfig, metrics)
		require.NoError(t, err)
		defer producer.Close()

		stats := producer.GetCircuitBreakerStats()
		assert.False(t, stats["enabled"].(bool))
	})

	t.Run("circuit_breaker_force_open_close", func(t *testing.T) {
		kafkaConfig := &config.KafkaConfig{
			Brokers:               []string{"localhost:9092"},
			TopicEvents:           "test.events",
			RequiredAcks:          1,
			Timeout:               30 * time.Second,
			MaxRetries:            3,
			RetryDelay:            1 * time.Second,
			CircuitBreakerEnabled: true,
			FailureThreshold:      2,
			SuccessThreshold:      2,
			TimeoutDuration:       5 * time.Second,
			ResetTimeout:          10 * time.Second,
		}

		metrics := metrics.NewMetrics()
		producer, err := kafka.NewProducer(kafkaConfig, metrics)
		require.NoError(t, err)
		defer producer.Close()

		// Initially closed
		assert.False(t, producer.IsCircuitBreakerOpen())
		assert.False(t, producer.IsCircuitBreakerHalfOpen())

		// Force open
		producer.ForceCircuitBreakerOpen()
		assert.True(t, producer.IsCircuitBreakerOpen())
		assert.False(t, producer.IsCircuitBreakerHalfOpen())

		// Force close
		producer.ForceCircuitBreakerClose()
		assert.False(t, producer.IsCircuitBreakerOpen())
		assert.False(t, producer.IsCircuitBreakerHalfOpen())
	})

	t.Run("circuit_breaker_basic_functionality", func(t *testing.T) {
		kafkaConfig := &config.KafkaConfig{
			Brokers:               []string{"localhost:9092"},
			TopicEvents:           "test.events",
			RequiredAcks:          1,
			Timeout:               30 * time.Second,
			MaxRetries:            3,
			RetryDelay:            1 * time.Second,
			CircuitBreakerEnabled: true,
			FailureThreshold:      2,
			SuccessThreshold:      2,
			TimeoutDuration:       5 * time.Second,
			ResetTimeout:          10 * time.Second,
		}

		metrics := metrics.NewMetrics()
		producer, err := kafka.NewProducer(kafkaConfig, metrics)
		require.NoError(t, err)
		defer producer.Close()

		// Initially closed
		stats := producer.GetCircuitBreakerStats()
		assert.Equal(t, "CLOSED", stats["state"])

		// Force open to simulate failures
		producer.ForceCircuitBreakerOpen()
		stats = producer.GetCircuitBreakerStats()
		assert.Equal(t, "OPEN", stats["state"])

		// Verify circuit breaker is open
		assert.True(t, producer.IsCircuitBreakerOpen())
		assert.False(t, producer.IsCircuitBreakerHalfOpen())

		// Force close
		producer.ForceCircuitBreakerClose()
		stats = producer.GetCircuitBreakerStats()
		assert.Equal(t, "CLOSED", stats["state"])
		assert.False(t, producer.IsCircuitBreakerOpen())
		assert.False(t, producer.IsCircuitBreakerHalfOpen())
	})

	t.Run("circuit_breaker_state_transitions", func(t *testing.T) {
		kafkaConfig := &config.KafkaConfig{
			Brokers:               []string{"localhost:9092"},
			TopicEvents:           "test.events",
			RequiredAcks:          1,
			Timeout:               30 * time.Second,
			MaxRetries:            3,
			RetryDelay:            1 * time.Second,
			CircuitBreakerEnabled: true,
			FailureThreshold:      2,
			SuccessThreshold:      2,
			TimeoutDuration:       5 * time.Second,
			ResetTimeout:          100 * time.Millisecond, // Short timeout for testing
		}

		metrics := metrics.NewMetrics()
		producer, err := kafka.NewProducer(kafkaConfig, metrics)
		require.NoError(t, err)
		defer producer.Close()

		// Test state transitions
		stats := producer.GetCircuitBreakerStats()
		assert.Equal(t, "CLOSED", stats["state"])

		// Force open
		producer.ForceCircuitBreakerOpen()
		stats = producer.GetCircuitBreakerStats()
		assert.Equal(t, "OPEN", stats["state"])

		// Force close
		producer.ForceCircuitBreakerClose()
		stats = producer.GetCircuitBreakerStats()
		assert.Equal(t, "CLOSED", stats["state"])
	})

	t.Run("circuit_breaker_with_event_publishing", func(t *testing.T) {
		kafkaConfig := &config.KafkaConfig{
			Brokers:               []string{"localhost:9092"},
			TopicEvents:           "test.events",
			RequiredAcks:          1,
			Timeout:               30 * time.Second,
			MaxRetries:            3,
			RetryDelay:            1 * time.Second,
			CircuitBreakerEnabled: true,
			FailureThreshold:      2,
			SuccessThreshold:      2,
			TimeoutDuration:       5 * time.Second,
			ResetTimeout:          10 * time.Second,
		}

		metrics := metrics.NewMetrics()
		producer, err := kafka.NewProducer(kafkaConfig, metrics)
		require.NoError(t, err)
		defer producer.Close()

		// Create a test event
		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Test",
			EventType:     "test.event",
			EventData:     models.JSON{"test": "data"},
			Status:        models.OutboxStatusPending,
		}

		// Initially circuit breaker should be closed
		assert.False(t, producer.IsCircuitBreakerOpen())

		// Publish event (should work in simulation mode)
		err = producer.PublishEvent(context.Background(), event)
		assert.NoError(t, err, "Event publishing should work in simulation mode")

		// Circuit breaker should still be closed
		assert.False(t, producer.IsCircuitBreakerOpen())
	})
}
