package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/mocks"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

func TestKafkaProducer(t *testing.T) {
	kafkaConfig := &config.KafkaConfig{
		Brokers:      []string{"localhost:9092"},
		TopicEvents:  "txstream.events",
		GroupID:      "test-consumer-group",
		RequiredAcks: 1,
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   1 * time.Second,
	}

	t.Run("successful_event_publication_with_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData: models.JSON{
				"order_id":     uuid.New().String(),
				"customer_id":  "customer-123",
				"order_number": "ORD-001",
				"total_amount": 150.00,
				"currency":     "BRL",
				"items_count":  2,
			},
			Status:     models.OutboxStatusPending,
			CreatedAt:  time.Now(),
			RetryCount: 0,
		}

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(nil).
			Once()

		err := mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)

	})

	t.Run("failed_event_publication_with_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData: models.JSON{
				"order_id":     uuid.New().String(),
				"customer_id":  "customer-123",
				"order_number": "ORD-001",
				"total_amount": 150.00,
				"currency":     "BRL",
				"items_count":  2,
			},
			Status:     models.OutboxStatusPending,
			CreatedAt:  time.Now(),
			RetryCount: 0,
		}

		expectedError := assert.AnError
		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(expectedError).
			Once()

		err := mockProducer.PublishEvent(context.Background(), event)
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
	})

	t.Run("producer_configuration_with_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		mockProducer.EXPECT().
			GetConfig().
			Return(kafkaConfig).
			Once()

		config := mockProducer.GetConfig()
		assert.Equal(t, kafkaConfig, config)
		assert.Equal(t, "txstream.events", config.TopicEvents)
		assert.Equal(t, "test-consumer-group", config.GroupID)
	})

	t.Run("producer_connection_status_with_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		mockProducer.EXPECT().
			IsConnected().
			Return(true).
			Once()

		isConnected := mockProducer.IsConnected()
		assert.True(t, isConnected)
	})

	t.Run("producer_close_with_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		mockProducer.EXPECT().
			Close().
			Return(nil).
			Once()

		err := mockProducer.Close()
		assert.NoError(t, err)
	})

	t.Run("producer_close_with_error_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		expectedError := assert.AnError
		mockProducer.EXPECT().
			Close().
			Return(expectedError).
			Once()

		err := mockProducer.Close()
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
	})

	t.Run("multiple_calls_with_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		events := make([]*models.OutboxEvent, 3)
		for i := 0; i < 3; i++ {
			events[i] = &models.OutboxEvent{
				ID:            uuid.New(),
				AggregateID:   uuid.New().String(),
				AggregateType: "Order",
				EventType:     "OrderCreated",
				EventData: models.JSON{
					"order_id":     uuid.New().String(),
					"customer_id":  "customer-123",
					"order_number": fmt.Sprintf("ORD-%03d", i+1),
					"total_amount": float64(100 + i*50),
					"currency":     "BRL",
					"items_count":  i + 1,
				},
				Status:     models.OutboxStatusPending,
				CreatedAt:  time.Now(),
				RetryCount: 0,
			}
		}

		for _, event := range events {
			mockProducer.EXPECT().
				PublishEvent(context.Background(), event).
				Return(nil).
				Once()
		}

		for _, event := range events {
			err := mockProducer.PublishEvent(context.Background(), event)
			assert.NoError(t, err)
		}
	})

	t.Run("run_and_return_with_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData: models.JSON{
				"order_id":     uuid.New().String(),
				"customer_id":  "customer-123",
				"order_number": "ORD-001",
				"total_amount": 150.00,
				"currency":     "BRL",
				"items_count":  2,
			},
			Status:     models.OutboxStatusPending,
			CreatedAt:  time.Now(),
			RetryCount: 0,
		}

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			RunAndReturn(func(ctx context.Context, evt *models.OutboxEvent) error {
				require.Equal(t, "Order", evt.AggregateType)
				require.Equal(t, "OrderCreated", evt.EventType)
				require.NotEmpty(t, evt.AggregateID)
				return nil
			}).
			Once()

		err := mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)
	})

	t.Run("run_with_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData: models.JSON{
				"order_id":     uuid.New().String(),
				"customer_id":  "customer-123",
				"order_number": "ORD-001",
				"total_amount": 150.00,
				"currency":     "BRL",
				"items_count":  2,
			},
			Status:     models.OutboxStatusPending,
			CreatedAt:  time.Now(),
			RetryCount: 0,
		}

		customLogicExecuted := false

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Run(func(ctx context.Context, evt *models.OutboxEvent) {
				customLogicExecuted = true
				assert.Equal(t, "Order", evt.AggregateType)
				assert.Equal(t, "OrderCreated", evt.EventType)
			}).
			Return(nil).
			Once()

		err := mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)
		assert.True(t, customLogicExecuted, "Custom logic should have been executed")
	})

	t.Run("event_with_metadata_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData: models.JSON{
				"order_id":     uuid.New().String(),
				"customer_id":  "customer-123",
				"order_number": "ORD-001",
				"total_amount": 150.00,
				"currency":     "BRL",
				"items_count":  2,
			},
			EventMetadata: models.JSON{
				"correlation_id": uuid.New().String(),
				"user_id":        "user-123",
				"source":         "api",
				"version":        "1.0",
			},
			Status:     models.OutboxStatusPending,
			CreatedAt:  time.Now(),
			RetryCount: 0,
		}

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			RunAndReturn(func(ctx context.Context, evt *models.OutboxEvent) error {
				require.NotNil(t, evt.EventMetadata)
				require.Equal(t, "api", evt.EventMetadata["source"])
				require.Equal(t, "1.0", evt.EventMetadata["version"])
				return nil
			}).
			Once()

		err := mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)
	})

	t.Run("context_cancellation_expecter", func(t *testing.T) {
		mockProducer := mocks.NewEventProducer(t)

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData: models.JSON{
				"order_id":     uuid.New().String(),
				"customer_id":  "customer-123",
				"order_number": "ORD-001",
				"total_amount": 150.00,
				"currency":     "BRL",
				"items_count":  2,
			},
			Status:     models.OutboxStatusPending,
			CreatedAt:  time.Now(),
			RetryCount: 0,
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		mockProducer.EXPECT().
			PublishEvent(ctx, event).
			Return(nil).
			Once()

		err := mockProducer.PublishEvent(ctx, event)
		assert.NoError(t, err)
	})
}
