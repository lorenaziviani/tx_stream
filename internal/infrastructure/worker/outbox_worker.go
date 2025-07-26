package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/kafka"
	"github.com/lorenaziviani/txstream/internal/infrastructure/metrics"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
)

// OutboxWorker processes outbox events and publishes them to Kafka
type OutboxWorker struct {
	config     *config.Config
	outboxRepo repositories.OutboxRepository
	producer   kafka.EventProducer
	metrics    *metrics.Metrics
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewOutboxWorker creates a new OutboxWorker instance
func NewOutboxWorker(cfg *config.Config, outboxRepo repositories.OutboxRepository, producer kafka.EventProducer) *OutboxWorker {
	metrics := metrics.NewMetrics()

	worker := &OutboxWorker{
		config:     cfg,
		outboxRepo: outboxRepo,
		producer:   producer,
		metrics:    metrics,
		stopChan:   make(chan struct{}),
	}

	if cfg.Metrics.Enabled {
		if err := metrics.StartMetricsServer(cfg.Metrics.Port, cfg.Metrics.Path); err != nil {
			log.Printf("Failed to start metrics server: %v", err)
		} else {
			log.Printf("Metrics server started on port %d at %s", cfg.Metrics.Port, cfg.Metrics.Path)
		}
	}

	metrics.SetWorkerPoolSize(cfg.Worker.PoolSize)

	return worker
}

// Start begins processing outbox events
func (w *OutboxWorker) Start(ctx context.Context) error {
	log.Printf("Starting OutboxWorker with pool size: %d, batch size: %d, interval: %v",
		w.config.Worker.PoolSize, w.config.Worker.BatchSize, w.config.Worker.Interval)

	w.wg.Add(1)
	go w.run(ctx)

	return nil
}

// Stop gracefully stops the worker
func (w *OutboxWorker) Stop() {
	log.Println("Stopping OutboxWorker...")
	close(w.stopChan)
	w.wg.Wait()
	log.Println("OutboxWorker stopped")
}

// run is the main processing loop
func (w *OutboxWorker) run(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.Worker.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, stopping worker")
			return
		case <-w.stopChan:
			log.Println("Stop signal received, stopping worker")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

// processBatch fetches and processes a batch of events
func (w *OutboxWorker) processBatch(ctx context.Context) {
	timer := w.metrics.Timer()

	events, err := w.outboxRepo.GetPendingEvents(ctx, w.config.Worker.BatchSize)
	if err != nil {
		log.Printf("Failed to fetch pending events: %v", err)
		w.metrics.RecordEventFailed("database_error", "unknown")
		return
	}

	w.metrics.SetEventsInQueue("pending", len(events))

	if len(events) == 0 {
		return
	}

	log.Printf("Processing batch of %d events", len(events))

	for _, event := range events {
		if err := w.processEvent(ctx, &event); err != nil {
			log.Printf("Failed to process event %s: %v", event.ID, err)
		}
	}

	w.metrics.RecordEventProcessingDuration("batch", timer.Duration())
}

// processEvent processes a single event
func (w *OutboxWorker) processEvent(ctx context.Context, event *models.OutboxEvent) error {
	ctx, timer := metrics.ContextWithTimer(ctx, w.metrics)
	defer func() {
		w.metrics.RecordEventProcessingDuration(event.EventType, timer.Duration())
	}()

	log.Printf("Processing event: %s, type: %s", event.ID, event.EventType)

	lockedEvent, err := w.outboxRepo.GetPendingEventForUpdate(ctx, event.ID.String())
	if err != nil {
		log.Printf("Failed to acquire lock for event %s: %v", event.ID, err)
		w.metrics.RecordEventFailed("lock_error", event.EventType)
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	if lockedEvent.Status == models.OutboxStatusPublished {
		log.Printf("Event %s already published, skipping", event.ID)
		w.metrics.RecordEventProcessed("already_published", event.EventType)
		return nil
	}

	if lockedEvent.Status == models.OutboxStatusFailed && lockedEvent.RetryCount >= w.config.Worker.MaxRetries {
		log.Printf("Event %s permanently failed after %d retries", event.ID, lockedEvent.RetryCount)
		w.metrics.RecordEventProcessed("permanently_failed", event.EventType)
		return nil
	}

	publishTimer := w.metrics.Timer()
	err = w.producer.PublishEvent(ctx, lockedEvent)
	publishDuration := publishTimer.Duration()

	if err != nil {
		log.Printf("Failed to publish event %s: %v", event.ID, err)

		w.metrics.RecordEventFailed("publish_error", event.EventType)
		w.metrics.RecordEventPublishingDuration(w.config.Kafka.TopicEvents, event.EventType, publishDuration)

		return w.handlePublishError(lockedEvent, err)
	}

	if err := w.outboxRepo.MarkAsPublished(ctx, lockedEvent.ID.String()); err != nil {
		log.Printf("Failed to mark event %s as published: %v", event.ID, err)
		w.metrics.RecordEventFailed("update_error", event.EventType)
		return fmt.Errorf("failed to mark as published: %w", err)
	}

	w.metrics.RecordEventProcessed("published", event.EventType)
	w.metrics.RecordEventPublished(w.config.Kafka.TopicEvents, event.EventType)
	w.metrics.RecordEventPublishingDuration(w.config.Kafka.TopicEvents, event.EventType, publishDuration)

	log.Printf("Successfully published event: %s", event.ID)
	return nil
}

// handlePublishError handles publish failures with retry logic
func (w *OutboxWorker) handlePublishError(event *models.OutboxEvent, err error) error {
	event.RetryCount++

	w.metrics.RecordEventRetried(fmt.Sprintf("%d", event.RetryCount), event.EventType)

	if event.RetryCount < w.config.Worker.MaxRetries {
		errorMsg := fmt.Sprintf("Failed to publish (attempt %d/%d): %v", event.RetryCount, w.config.Worker.MaxRetries+1, err)
		if updateErr := w.outboxRepo.MarkAsFailed(context.Background(), event.ID.String(), errorMsg); updateErr != nil {
			log.Printf("Failed to mark event %s as failed: %v", event.ID, updateErr)
			return fmt.Errorf("failed to update retry count: %w", updateErr)
		}

		log.Printf("Event %s failed, will retry (attempt %d/%d)", event.ID, event.RetryCount, w.config.Worker.MaxRetries+1)
		return fmt.Errorf("publish failed, will retry: %w", err)
	}

	errorMsg := fmt.Sprintf("Failed to publish after %d attempts: %v", w.config.Worker.MaxRetries+1, err)
	if updateErr := w.outboxRepo.MarkAsFailed(context.Background(), event.ID.String(), errorMsg); updateErr != nil {
		log.Printf("Failed to mark event %s as permanently failed: %v", event.ID, updateErr)
		return fmt.Errorf("failed to mark as permanently failed: %w", updateErr)
	}

	log.Printf("Event %s permanently failed after %d attempts", event.ID, event.RetryCount)
	return fmt.Errorf("publish failed permanently after %d attempts: %w", w.config.Worker.MaxRetries+1, err)
}

// GetMetrics returns the metrics instance
func (w *OutboxWorker) GetMetrics() *metrics.Metrics {
	return w.metrics
}
