package integration

import (
	"context"
	"fmt"
	"sync"
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

func TestWorkerPoolIntegration(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	outboxRepo := repositories.NewOutboxRepository(db)

	workerConfig := &config.WorkerConfig{
		PoolSize:   2,
		BatchSize:  5,
		Interval:   1 * time.Second,
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}

	t.Run("parallel_event_processing", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

		events := make([]*models.OutboxEvent, 6)
		for i := 0; i < 6; i++ {
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

		worker := &MockWorkerPool{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
			workerCount:   3,
		}

		worker.Start()

		for _, event := range events {
			worker.SendEvent(event)
		}

		worker.WaitForCompletion()

		worker.Stop()

		for _, event := range events {
			dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
			require.NoError(t, err)
			assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
			assert.NotNil(t, dbEvent.PublishedAt)
		}
	})

	t.Run("worker_pool_idempotency", func(t *testing.T) {
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

		worker := &MockWorkerPool{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
			workerCount:   2,
		}

		worker.Start()

		worker.SendEvent(event)

		worker.WaitForCompletion()

		// Send the same event again (should be skipped due to idempotency)
		worker.SendEvent(event)

		worker.WaitForCompletion()

		worker.Stop()

		// Verify event was marked as published only once
		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
		assert.NotNil(t, dbEvent.PublishedAt)
	})

	t.Run("worker_pool_concurrent_failures", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		mockProducer := mocks.NewEventProducer(t)
		defer mockProducer.AssertExpectations(t)

		events := make([]*models.OutboxEvent, 4)
		for i := 0; i < 4; i++ {
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

		expectedError := fmt.Errorf("mock producer failure")
		mockProducer.EXPECT().
			PublishEvent(context.Background(), events[0]).
			Return(expectedError).
			Once()
		mockProducer.EXPECT().
			PublishEvent(context.Background(), events[1]).
			Return(expectedError).
			Once()
		mockProducer.EXPECT().
			PublishEvent(context.Background(), events[2]).
			Return(nil).
			Once()
		mockProducer.EXPECT().
			PublishEvent(context.Background(), events[3]).
			Return(nil).
			Once()

		worker := &MockWorkerPool{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
			workerCount:   2,
		}

		worker.Start()

		for _, event := range events {
			worker.SendEvent(event)
		}

		worker.WaitForCompletion()

		worker.Stop()

		// Verify failed events
		dbEvent0, err := outboxRepo.GetByID(context.Background(), events[0].ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusFailed, dbEvent0.Status)
		assert.Equal(t, 1, dbEvent0.RetryCount)

		dbEvent1, err := outboxRepo.GetByID(context.Background(), events[1].ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusFailed, dbEvent1.Status)
		assert.Equal(t, 1, dbEvent1.RetryCount)

		// Verify successful events
		dbEvent2, err := outboxRepo.GetByID(context.Background(), events[2].ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent2.Status)

		dbEvent3, err := outboxRepo.GetByID(context.Background(), events[3].ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent3.Status)
	})

	t.Run("worker_pool_graceful_shutdown", func(t *testing.T) {
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

		worker := &MockWorkerPool{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
			workerCount:   2,
		}

		worker.Start()

		for _, event := range events {
			worker.SendEvent(event)
		}

		time.Sleep(100 * time.Millisecond)

		worker.Stop()

		for _, event := range events {
			dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
			require.NoError(t, err)
			assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
		}
	})
}

// MockWorkerPool simulates the worker pool for testing
type MockWorkerPool struct {
	outboxRepo    repositories.OutboxRepository
	kafkaProducer *mocks.EventProducer
	config        *config.WorkerConfig
	workerCount   int

	eventChan    chan *models.OutboxEvent
	workerWg     sync.WaitGroup
	shutdownChan chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
}

func (w *MockWorkerPool) Start() {
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.eventChan = make(chan *models.OutboxEvent, w.workerCount*2)
	w.shutdownChan = make(chan struct{})

	for i := 0; i < w.workerCount; i++ {
		w.workerWg.Add(1)
		go w.worker(i)
	}
}

func (w *MockWorkerPool) Stop() {
	w.cancel()
	close(w.shutdownChan)
	w.workerWg.Wait()
	close(w.eventChan)
}

func (w *MockWorkerPool) SendEvent(event *models.OutboxEvent) {
	select {
	case w.eventChan <- event:
		// Event sent
	case <-w.ctx.Done():
		// Context cancelled
	}
}

func (w *MockWorkerPool) WaitForCompletion() {
	time.Sleep(500 * time.Millisecond)
}

func (w *MockWorkerPool) worker(id int) {
	defer w.workerWg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.shutdownChan:
			return
		case event, ok := <-w.eventChan:
			if !ok {
				return
			}
			w.processEvent(event)
		}
	}
}

func (w *MockWorkerPool) processEvent(event *models.OutboxEvent) {
	dbEvent, err := w.outboxRepo.GetByID(context.Background(), event.ID.String())
	if err != nil {
		return
	}

	if dbEvent.Status == models.OutboxStatusPublished {
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

func (w *MockWorkerPool) publishEvent(event *models.OutboxEvent) error {
	return w.kafkaProducer.PublishEvent(context.Background(), event)
}

func (w *MockWorkerPool) handlePublishError(event *models.OutboxEvent, err error) {
	event.RetryCount++

	if event.RetryCount < w.config.MaxRetries {
		errorMsg := fmt.Sprintf("Failed to publish (attempt %d/%d): %v",
			event.RetryCount, w.config.MaxRetries, err)
		if markErr := w.outboxRepo.MarkAsFailed(context.Background(), event.ID.String(), errorMsg); markErr != nil {
			return
		}
		return
	}

	errorMsg := fmt.Sprintf("Failed to publish after %d attempts: %v",
		w.config.MaxRetries, err)

	if markErr := w.outboxRepo.MarkAsFailed(context.Background(), event.ID.String(), errorMsg); markErr != nil {
		return
	}
}
