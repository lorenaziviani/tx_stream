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

		producer, err := kafka.NewProducer(kafkaConfig)
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

		producer, err := kafka.NewProducer(kafkaConfig)
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

		producer, err := kafka.NewProducer(kafkaConfig)
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

		producer, err := kafka.NewProducer(kafkaConfig)
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
	})

	t.Run("circuit_breaker_with_mock_producer", func(t *testing.T) {
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

		producer, err := kafka.NewProducer(kafkaConfig)
		require.NoError(t, err)
		defer producer.Close()

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Test",
			EventType:     "TestEvent",
			EventData: models.JSON{
				"test": "data",
			},
			Status:    models.OutboxStatusPending,
			CreatedAt: time.Now(),
		}

		// Publish should succeed (simulation mode)
		err = producer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)

		// Circuit breaker should still be closed
		stats := producer.GetCircuitBreakerStats()
		assert.Equal(t, "CLOSED", stats["state"])
		assert.Equal(t, 1, stats["success_count"])
	})

	t.Run("circuit_breaker_stats_consistency", func(t *testing.T) {
		kafkaConfig := &config.KafkaConfig{
			Brokers:               []string{"localhost:9092"},
			TopicEvents:           "test.events",
			RequiredAcks:          1,
			Timeout:               30 * time.Second,
			MaxRetries:            3,
			RetryDelay:            1 * time.Second,
			CircuitBreakerEnabled: true,
			FailureThreshold:      3,
			SuccessThreshold:      2,
			TimeoutDuration:       5 * time.Second,
			ResetTimeout:          10 * time.Second,
		}

		producer, err := kafka.NewProducer(kafkaConfig)
		require.NoError(t, err)
		defer producer.Close()

		// Get initial stats
		initialStats := producer.GetCircuitBreakerStats()
		assert.Equal(t, "CLOSED", initialStats["state"])
		assert.Equal(t, 0, initialStats["failure_count"])
		assert.Equal(t, 0, initialStats["success_count"])
		assert.Equal(t, 3, initialStats["failure_threshold"])
		assert.Equal(t, 2, initialStats["success_threshold"])

		// Force open and check stats
		producer.ForceCircuitBreakerOpen()
		openStats := producer.GetCircuitBreakerStats()
		assert.Equal(t, "OPEN", openStats["state"])
		assert.Equal(t, 0, openStats["failure_count"])
		assert.Equal(t, 0, openStats["success_count"])

		// Force close and check stats
		producer.ForceCircuitBreakerClose()
		closedStats := producer.GetCircuitBreakerStats()
		assert.Equal(t, "CLOSED", closedStats["state"])
		assert.Equal(t, 0, closedStats["failure_count"])
		assert.Equal(t, 0, closedStats["success_count"])
	})
}
