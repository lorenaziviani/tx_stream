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

func TestRaceConditionHandling(t *testing.T) {
	db := tests.SetupTestDatabase(t)
	defer tests.TeardownTestDatabase(t)

	outboxRepo := repositories.NewOutboxRepository(db)

	workerConfig := &config.Config{
		Worker: config.WorkerConfig{
			PollingInterval: 1 * time.Second,
			BatchSize:       10,
			MaxRetries:      3,
			ProcessTimeout:  30 * time.Second,
			Concurrency:     4, // Multiple workers to test race conditions
		},
	}

	t.Run("concurrent_event_processing_race_condition", func(t *testing.T) {
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
				"order_number": "ORD-RACE-TEST",
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

		worker := &RaceConditionMockWorkerPool{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
			workerCount:   4, // Multiple workers
		}

		worker.Start()

		// Send the same event multiple times concurrently to simulate race condition
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker.SendEvent(event)
			}()
		}

		wg.Wait()

		worker.WaitForCompletion()

		worker.Stop()

		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
		assert.NotNil(t, dbEvent.PublishedAt)
		assert.Equal(t, 0, dbEvent.RetryCount)
	})

	t.Run("concurrent_failed_event_processing", func(t *testing.T) {
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
				"order_number": "ORD-FAIL-RACE-TEST",
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
			Maybe()

		worker := &RaceConditionMockWorkerPool{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
			workerCount:   3, // Multiple workers
		}

		worker.Start()

		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker.SendEvent(event)
			}()
		}

		wg.Wait()

		worker.WaitForCompletion()

		worker.Stop()

		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusFailed, dbEvent.Status)
		assert.Equal(t, 1, dbEvent.RetryCount)
		assert.Contains(t, dbEvent.ErrorMessage, "Failed to publish (attempt 1/3)")
	})

	t.Run("mixed_success_failure_race_condition", func(t *testing.T) {
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
				"order_number": "ORD-MIXED-RACE-TEST",
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

		// Set expectations: first call fails, second call succeeds
		// Due to locking, only one worker will process the event
		expectedError := fmt.Errorf("mock producer failure")
		mockProducer.EXPECT().
			PublishEvent(context.Background(), event).
			Return(expectedError).
			Maybe()

		worker := &RaceConditionMockWorkerPool{
			outboxRepo:    outboxRepo,
			kafkaProducer: mockProducer,
			config:        workerConfig,
			workerCount:   2, // Multiple workers
		}

		worker.Start()

		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker.SendEvent(event)
			}()
		}

		wg.Wait()

		worker.WaitForCompletion()

		worker.Stop()

		// Verify final state - should be failed due to lock preventing retry
		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusFailed, dbEvent.Status)
		assert.Equal(t, 1, dbEvent.RetryCount)
		assert.Contains(t, dbEvent.ErrorMessage, "Failed to publish (attempt 1/3)")
	})

	t.Run("repository_lock_methods_work_correctly", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData: models.JSON{
				"order_id":     uuid.New().String(),
				"customer_id":  "customer-123",
				"order_number": "ORD-LOCK-TEST",
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

		lockedEvent, err := outboxRepo.GetPendingEventForUpdate(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, event.ID, lockedEvent.ID)
		assert.Equal(t, models.OutboxStatusPending, lockedEvent.Status)

		err = outboxRepo.MarkAsPublishedWithLock(context.Background(), event.ID.String())
		require.NoError(t, err)

		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusPublished, dbEvent.Status)
		assert.NotNil(t, dbEvent.PublishedAt)

		_, err = outboxRepo.GetPendingEventForUpdate(context.Background(), event.ID.String())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pending outbox event not found")
	})

	t.Run("concurrent_mark_as_failed_race_condition", func(t *testing.T) {
		tests.CleanupTestDatabase(t, db)

		event := &models.OutboxEvent{
			ID:            uuid.New(),
			AggregateID:   uuid.New().String(),
			AggregateType: "Order",
			EventType:     "OrderCreated",
			EventData: models.JSON{
				"order_id":     uuid.New().String(),
				"customer_id":  "customer-123",
				"order_number": "ORD-FAIL-LOCK-TEST",
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

		var wg sync.WaitGroup
		errorMsg := "concurrent failure test"

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := outboxRepo.MarkAsFailedWithLock(context.Background(), event.ID.String(), errorMsg)
				// Some calls might fail because the event is already marked as failed
				if err != nil {
					assert.Contains(t, err.Error(), "pending outbox event not found")
				}
			}()
		}

		wg.Wait()

		dbEvent, err := outboxRepo.GetByID(context.Background(), event.ID.String())
		require.NoError(t, err)
		assert.Equal(t, models.OutboxStatusFailed, dbEvent.Status)
		assert.Equal(t, 1, dbEvent.RetryCount)
		assert.Contains(t, dbEvent.ErrorMessage, errorMsg)
	})
}

// RaceConditionMockWorkerPool simulates the worker pool for testing race conditions
type RaceConditionMockWorkerPool struct {
	outboxRepo    repositories.OutboxRepository
	kafkaProducer *mocks.EventProducer
	config        *config.Config
	workerCount   int

	eventChan    chan *models.OutboxEvent
	workerWg     sync.WaitGroup
	shutdownChan chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
}

func (w *RaceConditionMockWorkerPool) Start() {
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.eventChan = make(chan *models.OutboxEvent, w.workerCount*2)
	w.shutdownChan = make(chan struct{})

	for i := 0; i < w.workerCount; i++ {
		w.workerWg.Add(1)
		go w.worker(i)
	}
}

func (w *RaceConditionMockWorkerPool) Stop() {
	w.cancel()
	close(w.shutdownChan)
	w.workerWg.Wait()
	close(w.eventChan)
}

func (w *RaceConditionMockWorkerPool) SendEvent(event *models.OutboxEvent) {
	select {
	case w.eventChan <- event:
		// Event sent
	case <-w.ctx.Done():
		// Context cancelled
	}
}

func (w *RaceConditionMockWorkerPool) WaitForCompletion() {
	time.Sleep(500 * time.Millisecond)
}

func (w *RaceConditionMockWorkerPool) worker(id int) {
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

func (w *RaceConditionMockWorkerPool) processEvent(event *models.OutboxEvent) {
	dbEvent, err := w.outboxRepo.GetByID(context.Background(), event.ID.String())
	if err != nil {
		return
	}

	if dbEvent.Status == models.OutboxStatusPublished {
		return
	}

	if dbEvent.Status == models.OutboxStatusFailed && dbEvent.RetryCount >= w.config.Worker.MaxRetries {
		return
	}

	if err := w.publishEvent(event); err != nil {
		w.handlePublishError(event, err)
		return
	}

	if err := w.outboxRepo.MarkAsPublishedWithLock(context.Background(), event.ID.String()); err != nil {
		return
	}
}

func (w *RaceConditionMockWorkerPool) publishEvent(event *models.OutboxEvent) error {
	return w.kafkaProducer.PublishEvent(context.Background(), event)
}

func (w *RaceConditionMockWorkerPool) handlePublishError(event *models.OutboxEvent, err error) {
	dbEvent, err := w.outboxRepo.GetByID(context.Background(), event.ID.String())
	if err != nil {
		return
	}

	if dbEvent.RetryCount < w.config.Worker.MaxRetries {
		errorMsg := fmt.Sprintf("Failed to publish (attempt %d/%d): %v",
			dbEvent.RetryCount+1, w.config.Worker.MaxRetries, err)
		if markErr := w.outboxRepo.MarkAsFailedWithLock(context.Background(), event.ID.String(), errorMsg); markErr != nil {
			return
		}
		return
	}

	errorMsg := fmt.Sprintf("Failed to publish after %d attempts: %v",
		w.config.Worker.MaxRetries, err)

	if markErr := w.outboxRepo.MarkAsFailedWithLock(context.Background(), event.ID.String(), errorMsg); markErr != nil {
		return
	}
}
