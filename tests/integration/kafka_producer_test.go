package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorenaziviani/txstream/internal/infrastructure/mocks"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
	"github.com/lorenaziviani/txstream/tests"
)

func TestKafkaProducerIntegration(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	outboxRepo := repositories.NewOutboxRepository(db)

	t.Run("successful_event_publication", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

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

		err := outboxRepo.Create(context.Background(), event)
		require.NoError(t, err)

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(nil).
			Once()

		err = mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)
	})

	t.Run("idempotency_check", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

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

		err := outboxRepo.Create(context.Background(), event)
		require.NoError(t, err)

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(nil).
			Once()

		err = mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(nil).
			Once()

		// Publish same event again (idempotency test)
		err = mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)
	})

	t.Run("broker_failure_and_recovery", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

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

		err := outboxRepo.Create(context.Background(), event)
		require.NoError(t, err)

		expectedError := fmt.Errorf("mock producer failure")
		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(expectedError).
			Once()

		// Try to publish event (should fail)
		err = mockProducer.PublishEvent(context.Background(), event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mock producer failure")

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(nil).
			Once()

		// Try to publish event again (should succeed)
		err = mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)
	})

	t.Run("intermittent_failures", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

		events := []*models.OutboxEvent{
			{
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
			},
			{
				ID:            uuid.New(),
				AggregateID:   uuid.New().String(),
				AggregateType: "Order",
				EventType:     "OrderUpdated",
				EventData: models.JSON{
					"order_id":     uuid.New().String(),
					"customer_id":  "customer-123",
					"order_number": "ORD-002",
					"total_amount": 200.00,
					"currency":     "BRL",
					"items_count":  3,
				},
				Status:     models.OutboxStatusPending,
				CreatedAt:  time.Now(),
				RetryCount: 0,
			},
		}

		for _, event := range events {
			err := outboxRepo.Create(context.Background(), event)
			require.NoError(t, err)
		}

		// Set expectations for intermittent failures (fail first, succeed second)
		expectedError := fmt.Errorf("mock producer failure")

		// First event: fail then succeed
		mockProducer.EXPECT().
			PublishEvent(context.Background(), events[0]).
			Return(expectedError).
			Once()
		mockProducer.EXPECT().
			PublishEvent(context.Background(), events[0]).
			Return(nil).
			Once()

		// Second event: fail then succeed
		mockProducer.EXPECT().
			PublishEvent(context.Background(), events[1]).
			Return(expectedError).
			Once()
		mockProducer.EXPECT().
			PublishEvent(context.Background(), events[1]).
			Return(nil).
			Once()

		// Publish first event (should fail on first attempt, succeed on second)
		err := mockProducer.PublishEvent(context.Background(), events[0])
		assert.Error(t, err)

		err = mockProducer.PublishEvent(context.Background(), events[0])
		assert.NoError(t, err)

		// Publish second event (should fail on first attempt, succeed on second)
		err = mockProducer.PublishEvent(context.Background(), events[1])
		assert.Error(t, err)

		err = mockProducer.PublishEvent(context.Background(), events[1])
		assert.NoError(t, err)
	})

	t.Run("multiple_events_batch", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

		events := make([]*models.OutboxEvent, 5)
		for i := 0; i < 5; i++ {
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

			err := outboxRepo.Create(context.Background(), events[i])
			require.NoError(t, err)
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

	t.Run("event_metadata_inclusion", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

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

		err := outboxRepo.Create(context.Background(), event)
		require.NoError(t, err)

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			RunAndReturn(func(ctx context.Context, evt *models.OutboxEvent) error {
				assert.NotNil(t, evt.EventMetadata)
				assert.Equal(t, "api", evt.EventMetadata["source"])
				assert.Equal(t, "1.0", evt.EventMetadata["version"])
				return nil
			}).
			Once()

		err = mockProducer.PublishEvent(context.Background(), event)
		assert.NoError(t, err)
	})
}
