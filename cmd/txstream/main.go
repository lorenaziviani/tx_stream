package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/lorenaziviani/txstream/internal/application/usecases"
	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/database"
	"github.com/lorenaziviani/txstream/internal/infrastructure/handlers"
	"github.com/lorenaziviani/txstream/internal/infrastructure/repositories"
)

var (
	cfg *config.Config
)

func main() {
	// Load configuration
	var err error
	cfg, err = config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	if err := database.InitializeDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDatabase()

	// Get database connection
	db := database.GetDB()

	// Initialize repositories
	orderRepo := repositories.NewOrderRepository(db)
	outboxRepo := repositories.NewOutboxRepository(db)

	// Initialize use cases
	orderUseCase := usecases.NewOrderUseCase(orderRepo, outboxRepo, db)

	// Initialize handlers
	orderHandler := handlers.NewOrderHandler(orderUseCase)

	// Setup router
	router := mux.NewRouter()
	router.Use(loggingMiddleware)

	// Health and readiness endpoints
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")
	router.HandleFunc("/ready", readinessCheckHandler).Methods("GET")

	// API routes
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	setupAPIRoutes(apiRouter, orderHandler)

	// Create server
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupAPIRoutes(router *mux.Router, orderHandler *handlers.OrderHandler) {
	router.HandleFunc("/orders", orderHandler.CreateOrderHandler).Methods("POST")
	router.HandleFunc("/orders", orderHandler.ListOrdersHandler).Methods("GET")
	router.HandleFunc("/orders/{id}", orderHandler.GetOrderByIDHandler).Methods("GET")
	router.HandleFunc("/orders/number/{orderNumber}", orderHandler.GetOrderByNumberHandler).Methods("GET")

	router.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Events endpoint - em desenvolvimento"}`))
	}).Methods("GET")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
		"config": map[string]interface{}{
			"server": map[string]interface{}{
				"host": cfg.Server.Host,
				"port": cfg.Server.Port,
			},
			"database": map[string]interface{}{
				"host": cfg.Database.Host,
				"port": cfg.Database.Port,
				"name": cfg.Database.Name,
			},
			"kafka": map[string]interface{}{
				"enabled": cfg.Kafka.IsKafkaEnabled(),
				"brokers": cfg.Kafka.GetKafkaBrokers(),
			},
		},
	}

	// Add database health check
	if err := database.HealthCheck(); err != nil {
		health["status"] = "unhealthy"
		health["database_error"] = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		health["database_stats"] = database.GetConnectionStats()
	}

	json.NewEncoder(w).Encode(health)
}

func readinessCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	readiness := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().UTC(),
		"services": map[string]interface{}{
			"database": "ready",
			"kafka":    "disabled", // Kafka is currently disabled
		},
	}

	// Check database readiness
	if err := database.HealthCheck(); err != nil {
		readiness["status"] = "not_ready"
		readiness["services"].(map[string]interface{})["database"] = "not_ready"
		readiness["database_error"] = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(readiness)
}
