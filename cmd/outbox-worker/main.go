package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/database"
	"github.com/lorenaziviani/txstream/internal/infrastructure/kafka"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
)

type OutboxWorker struct {
	outboxRepo    repositories.OutboxRepository
	kafkaProducer kafka.EventProducer
	config        *config.Config
	ctx           context.Context
	cancel        context.CancelFunc

	eventChan    chan *models.OutboxEvent
	workerWg     sync.WaitGroup
	workerCount  int
	shutdownChan chan struct{}
}

func NewOutboxWorker(outboxRepo repositories.OutboxRepository, kafkaProducer kafka.EventProducer, cfg *config.Config) *OutboxWorker {
	ctx, cancel := context.WithCancel(context.Background())

	workerCount := cfg.Worker.Concurrency
	if workerCount <= 0 {
		workerCount = 1
	}

	return &OutboxWorker{
		outboxRepo:    outboxRepo,
		kafkaProducer: kafkaProducer,
		config:        cfg,
		ctx:           ctx,
		cancel:        cancel,
		eventChan:     make(chan *models.OutboxEvent, workerCount*2), // Buffer for 2x worker count
		workerCount:   workerCount,
		shutdownChan:  make(chan struct{}),
	}
}

// Start starts the outbox worker with worker pool
func (w *OutboxWorker) Start() {
	log.Println("Starting Outbox Worker...")
	log.Printf("Configuration: polling=%v, batch_size=%d, max_retries=%d, concurrency=%d",
		w.config.Worker.PollingInterval, w.config.Worker.BatchSize, w.config.Worker.MaxRetries, w.workerCount)

	if w.kafkaProducer.IsConnected() {
		log.Printf("Kafka producer connected to brokers: %v", w.kafkaProducer.GetConfig().GetKafkaBrokers())
	} else {
		log.Println("Kafka producer not connected - running in simulation mode")
	}

	w.startWorkerPool()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go w.pollingLoop()

	<-sigChan
	log.Println("Shutdown signal received, stopping worker...")
	w.Stop()
}

// Stop stops the outbox worker
func (w *OutboxWorker) Stop() {
	w.cancel()

	close(w.shutdownChan)

	log.Printf("Waiting for %d workers to finish...", w.workerCount)
	w.workerWg.Wait()

	close(w.eventChan)

	if err := w.kafkaProducer.Close(); err != nil {
		log.Printf("Error closing Kafka producer: %v", err)
	}

	log.Println("Outbox Worker stopped")
}

// startWorkerPool starts the worker pool
func (w *OutboxWorker) startWorkerPool() {
	log.Printf("Starting worker pool with %d workers", w.workerCount)

	for i := 0; i < w.workerCount; i++ {
		w.workerWg.Add(1)
		go w.worker(i)
	}
}

// worker is a single worker goroutine
func (w *OutboxWorker) worker(id int) {
	defer w.workerWg.Done()

	log.Printf("Worker %d started", id)

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("Worker %d: context cancelled", id)
			return
		case <-w.shutdownChan:
			log.Printf("Worker %d: shutdown signal received", id)
			return
		case event, ok := <-w.eventChan:
			if !ok {
				log.Printf("Worker %d: event channel closed", id)
				return
			}

			log.Printf("Worker %d: processing event %s", id, event.ID)
			w.processEvent(event)
		}
	}
}

// pollingLoop starts the polling loop
func (w *OutboxWorker) pollingLoop() {
	ticker := time.NewTicker(w.config.Worker.PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Println("Polling loop stopped")
			return
		case <-ticker.C:
			w.processPendingEvents()
		}
	}
}

// processPendingEvents processes the pending events by sending them to the worker pool
func (w *OutboxWorker) processPendingEvents() {
	events, err := w.outboxRepo.GetPendingEvents(w.ctx, w.config.Worker.BatchSize)
	if err != nil {
		log.Printf("Error fetching pending events: %v", err)
		return
	}

	if len(events) == 0 {
		log.Println("No pending events found")
		return
	}

	log.Printf("Found %d pending events, sending to worker pool", len(events))

	for _, event := range events {
		select {
		case <-w.ctx.Done():
			log.Println("Context cancelled, stopping event distribution")
			return
		case w.eventChan <- &event:
			// Event sent to worker pool
		default:
			// Channel is full, log warning but continue
			log.Printf("Warning: event channel is full, event %s will be processed in next cycle", event.ID)
		}
	}
}

// processEvent processes an event
func (w *OutboxWorker) processEvent(event *models.OutboxEvent) {
	log.Printf("Processing event: ID=%s, Type=%s, AggregateID=%s",
		event.ID, event.EventType, event.AggregateID)

	w.printEventDetails(event)

	if err := w.publishEvent(event); err != nil {
		log.Printf("Failed to publish event %s: %v", event.ID, err)
		w.handlePublishError(event, err)
		return
	}

	if err := w.outboxRepo.MarkAsPublished(w.ctx, event.ID.String()); err != nil {
		log.Printf("Failed to mark event %s as published: %v", event.ID, err)
		return
	}

	log.Printf("Event %s processed successfully", event.ID)
}

// publishEvent publishes an event to Kafka
func (w *OutboxWorker) publishEvent(event *models.OutboxEvent) error {
	if !w.kafkaProducer.IsConnected() {
		log.Printf("Kafka not connected, simulating event publication for %s", event.ID)
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	ctx, cancel := context.WithTimeout(w.ctx, w.config.Worker.ProcessTimeout)
	defer cancel()

	return w.kafkaProducer.PublishEvent(ctx, event)
}

// handlePublishError handles publish errors
func (w *OutboxWorker) handlePublishError(event *models.OutboxEvent, err error) {
	if event.RetryCount < w.config.Worker.MaxRetries {
		log.Printf("Event %s will be retried (attempt %d/%d)",
			event.ID, event.RetryCount+1, w.config.Worker.MaxRetries)
		return
	}

	errorMsg := fmt.Sprintf("Failed to publish after %d attempts: %v",
		w.config.Worker.MaxRetries, err)

	if markErr := w.outboxRepo.MarkAsFailed(w.ctx, event.ID.String(), errorMsg); markErr != nil {
		log.Printf("Failed to mark event %s as failed: %v", event.ID, markErr)
	}
}

// printEventDetails prints the event details
func (w *OutboxWorker) printEventDetails(event *models.OutboxEvent) {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("EVENT DETAILS\n")
	fmt.Printf("ID: %s\n", event.ID)
	fmt.Printf("Aggregate ID: %s\n", event.AggregateID)
	fmt.Printf("Aggregate Type: %s\n", event.AggregateType)
	fmt.Printf("Event Type: %s\n", event.EventType)
	fmt.Printf("Status: %s\n", event.Status)
	fmt.Printf("Created At: %s\n", event.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Retry Count: %d\n", event.RetryCount)

	if event.EventData != nil {
		fmt.Printf("Event Data:\n")
		eventDataJSON, _ := json.MarshalIndent(event.EventData, "", "  ")
		fmt.Println(string(eventDataJSON))
	}

	if event.EventMetadata != nil {
		fmt.Printf("Event Metadata:\n")
		metadataJSON, _ := json.MarshalIndent(event.EventMetadata, "", "  ")
		fmt.Println(string(metadataJSON))
	}
	fmt.Println(strings.Repeat("=", 80))
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := database.InitializeDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDatabase()

	db := database.GetDB()

	outboxRepo := repositories.NewOutboxRepository(db)

	kafkaProducer, err := kafka.NewProducer(&cfg.Kafka)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}

	worker := NewOutboxWorker(outboxRepo, kafkaProducer, cfg)
	worker.Start()
}
