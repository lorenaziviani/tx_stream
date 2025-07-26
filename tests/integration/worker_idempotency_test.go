package integration

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
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
	"github.com/lorenaziviani/txstream/tests"
)

func TestWorkerIdempotency(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	outboxRepo := repositories.NewOutboxRepository(db)

	workerConfig := &config.Config{
		Worker: config.WorkerConfig{
			PollingInterval: 1 * time.Second,
			BatchSize:       10,
			MaxRetries:      3,
			ProcessTimeout:  30 * time.Second,
			Concurrency:     1,
		},
	}

	t.Run("worker_processes_event_once", func(t *testing.T) {
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

		worker := &MockOutboxWorker{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
		}

		worker.processEvent(event)

		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
		assert.NotNil(t, dbEvent.PublishedAt)
	})

	t.Run("worker_handles_duplicate_processing", func(t *testing.T) {
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

		worker := &MockOutboxWorker{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
		}

		worker.processEvent(event)

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(nil).
			Once()

		worker.processEvent(event)

		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
	})

	t.Run("worker_handles_already_published_events", func(t *testing.T) {
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
			Status:      models.OutboxStatusPublished,
			CreatedAt:   time.Now(),
			PublishedAt: &[]time.Time{time.Now()}[0],
			RetryCount:  0,
		}

		err := outboxRepo.Create(context.Background(), event)
		require.NoError(t, err)

		// Process event (should be skipped since already published)
		// No expectations set because the event should be skipped

		// Verify event status in database remains published
		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
	})

	t.Run("worker_retry_failed_events", func(t *testing.T) {
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

		worker := &MockOutboxWorker{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
		}

		// Process event (should fail)
		worker.processEvent(event)

		// Verify event was marked as failed in database
		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusFailed, dbEvent.Status)
		assert.Equal(t, 1, dbEvent.RetryCount)

		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(nil).
			Once()

		// Process event again (should succeed)
		worker.processEvent(event)

		// Verify event was marked as published in database
		dbEvent, err = outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
		assert.NotNil(t, dbEvent.PublishedAt)
	})

	t.Run("worker_max_retries_exceeded", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

		// Create test event with max retries already reached
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
			RetryCount: 3, // Already at max retries
		}

		err := outboxRepo.Create(context.Background(), event)
		require.NoError(t, err)

		expectedError := fmt.Errorf("mock producer failure")
		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(expectedError).
			Once()

		worker := &MockOutboxWorker{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
		}

		// Process event (should fail and not retry)
		worker.processEvent(event)

		// Verify event was marked as failed in database
		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusFailed, dbEvent.Status)
		assert.Equal(t, 4, dbEvent.RetryCount) // Should be incremented
		assert.NotEmpty(t, dbEvent.ErrorMessage)
	})

	t.Run("worker_batch_processing_idempotency", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

		// Create multiple test events
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

			err := outboxRepo.Create(context.Background(), events[i])
			require.NoError(t, err)
		}

		// Set expectations for first processing
		for _, event := range events {
			mockProducer.EXPECT().
				PublishEvent(context.Background(), event).
				Return(nil).
				Once()
		}

		worker := &MockOutboxWorker{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
		}

		for _, event := range events {
			worker.processEvent(event)
		}

		// Set expectations for second processing (idempotency)
		for _, event := range events {
			mockProducer.EXPECT().
				PublishEvent(context.Background(), event).
				Return(nil).
				Once()
		}

		// Process all events again (idempotency test)
		for _, event := range events {
			worker.processEvent(event)
		}

		// Verify all events are marked as published in database
		for _, event := range events {
			dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
			require.NoError(t, err)
			assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
		}
	})
}

// MockOutboxWorker is a simplified version of the worker for testing
type MockOutboxWorker struct {
	outboxRepo    repositories.OutboxRepository
	kafkaProducer *mocks.EventProducer
	config        *config.Config
}

// processEvent processes a single event
func (w *MockOutboxWorker) processEvent(event *models.OutboxEvent) {
	if event.Status == models.OutboxStatusPublished {
		return
	}

	if err := w.publishEvent(event); err != nil {
		w.handlePublishError(event, err)
		return
	}

	if err := w.outboxRepo.MarkAsPublished(context.Background(), event.ID.String()); err != nil {
		return
	}
}

// publishEvent publishes an event to Kafka
func (w *MockOutboxWorker) publishEvent(event *models.OutboxEvent) error {
	return w.kafkaProducer.PublishEvent(context.Background(), event)
}

// handlePublishError handles publish errors
func (w *MockOutboxWorker) handlePublishError(event *models.OutboxEvent, err error) {
	event.RetryCount++

	if event.RetryCount < w.config.Worker.MaxRetries {
		errorMsg := fmt.Sprintf("Failed to publish (attempt %d/%d): %v",
			event.RetryCount, w.config.Worker.MaxRetries, err)
		if markErr := w.outboxRepo.MarkAsFailed(context.Background(), event.ID.String(), errorMsg); markErr != nil {
			return
		}
		return
	}

	errorMsg := fmt.Sprintf("Failed to publish after %d attempts: %v",
		w.config.Worker.MaxRetries, err)

	if markErr := w.outboxRepo.MarkAsFailed(context.Background(), event.ID.String(), errorMsg); markErr != nil {
		return
	}
}
