package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/database"
	"github.com/lorenaziviani/txstream/internal/infrastructure/kafka"
	"github.com/lorenaziviani/txstream/internal/infrastructure/metrics"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
	"github.com/lorenaziviani/txstream/internal/infrastructure/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	outboxRepo := repositories.NewOutboxRepository(db)

	metrics := metrics.NewMetrics()

	kafkaProducer, err := kafka.NewProducer(&cfg.Kafka, metrics)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}

	outboxWorker := worker.NewOutboxWorker(cfg, outboxRepo, kafkaProducer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Printf("Starting outbox worker with %d workers, batch size %d, interval %v",
			cfg.Worker.PoolSize, cfg.Worker.BatchSize, cfg.Worker.Interval)
		outboxWorker.Start(ctx)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping worker...")

	outboxWorker.Stop()
	log.Println("Outbox Worker stopped")
}
