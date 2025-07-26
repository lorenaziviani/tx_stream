package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lorenaziviani/txstream/internal/infrastructure/database"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
)

const (
	defaultPollingInterval = 5 * time.Second
	defaultBatchSize       = 10
	defaultMaxRetries      = 3
)

type OutboxWorker struct {
	outboxRepo      repositories.OutboxRepository
	pollingInterval time.Duration
	batchSize       int
	maxRetries      int
	ctx             context.Context
	cancel          context.CancelFunc
}

func NewOutboxWorker(outboxRepo repositories.OutboxRepository) *OutboxWorker {
	ctx, cancel := context.WithCancel(context.Background())

	return &OutboxWorker{
		outboxRepo:      outboxRepo,
		pollingInterval: defaultPollingInterval,
		batchSize:       defaultBatchSize,
		maxRetries:      defaultMaxRetries,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start starts the outbox worker
func (w *OutboxWorker) Start() {
	log.Println("Starting Outbox Worker...")
	log.Printf("Configuration: polling=%v, batch_size=%d, max_retries=%d",
		w.pollingInterval, w.batchSize, w.maxRetries)

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
	log.Println("Outbox Worker stopped")
}

// pollingLoop starts the polling loop
func (w *OutboxWorker) pollingLoop() {
	ticker := time.NewTicker(w.pollingInterval)
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

// processPendingEvents processes the pending events
func (w *OutboxWorker) processPendingEvents() {
	events, err := w.outboxRepo.GetPendingEvents(w.ctx, w.batchSize)
	if err != nil {
		log.Printf("Error fetching pending events: %v", err)
		return
	}

	if len(events) == 0 {
		log.Println("No pending events found")
		return
	}

	log.Printf("Found %d pending events", len(events))

	for _, event := range events {
		w.processEvent(event)
	}
}

// processEvent processes an event
func (w *OutboxWorker) processEvent(event models.OutboxEvent) {
	log.Printf("Processing event: ID=%s, Type=%s, AggregateID=%s",
		event.ID, event.EventType, event.AggregateID)

	w.printEventDetails(event)

	w.simulateEventProcessing()

	w.markEventAsPublished(event)
}

// printEventDetails prints the event details
func (w *OutboxWorker) printEventDetails(event models.OutboxEvent) {
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

// simulateEventProcessing simulates the event processin
func (w *OutboxWorker) simulateEventProcessing() {
	log.Printf("Simulating event publishing to Kafka...")

	time.Sleep(100 * time.Millisecond)

	log.Printf("Event processing simulated successfully")
}

// markEventAsPublished marks the event as published
func (w *OutboxWorker) markEventAsPublished(event models.OutboxEvent) {
	log.Printf("Marking event %s as published", event.ID)

	// TODO: Implement actual status update in the repository
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, using system environment variables")
	}

	if err := database.InitializeDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDatabase()

	db := database.GetDB()

	outboxRepo := repositories.NewOutboxRepository(db)

	worker := NewOutboxWorker(outboxRepo)
	worker.Start()
}
