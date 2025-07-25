package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"github.com/lorenaziviani/txstream/internal/infrastructure/database"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("File .env not found, using system environment variables")
	}

	if err := database.InitializeDatabase(); err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer database.CloseDatabase()

	router := mux.NewRouter()

	router.Use(loggingMiddleware)

	router.HandleFunc("/health", healthCheckHandler).Methods("GET")
	router.HandleFunc("/ready", readinessCheckHandler).Methods("GET")

	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	setupAPIRoutes(apiRouter)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("TxStream starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	<-stop
	log.Println("Received shutdown signal, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server shutdown successfully")
}

func setupAPIRoutes(router *mux.Router) {
	// TODO: Implement routes
	router.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Orders endpoint - em desenvolvimento"}`))
	}).Methods("POST", "GET")

	router.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Events endpoint - em desenvolvimento"}`))
	}).Methods("GET")
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy", "service": "txstream", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`))
}

func readinessCheckHandler(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	if db == nil {
		http.Error(w, `{"status": "not ready", "error": "database not connected"}`, http.StatusServiceUnavailable)
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		http.Error(w, `{"status": "not ready", "error": "database connection error"}`, http.StatusServiceUnavailable)
		return
	}

	if err := sqlDB.Ping(); err != nil {
		http.Error(w, `{"status": "not ready", "error": "database ping failed"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ready", "service": "txstream", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		log.Printf(
			"%s %s %s %v",
			r.Method,
			r.RequestURI,
			r.RemoteAddr,
			time.Since(start),
		)
	})
}
