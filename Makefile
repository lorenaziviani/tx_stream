.PHONY: help build run test clean migrate deps docker-build docker-run

BINARY_NAME=txstream
BUILD_DIR=build
DOCKER_IMAGE=txstream:latest

help: ## Show this help
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

deps: ## Install project dependencies
	go mod download
	go mod tidy

build: ## Compile the project
	@echo "🔨 Compiling TxStream..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/txstream

build-worker: ## Compile the outbox worker
	@echo "🔨 Compiling Outbox Worker..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/outbox-worker ./cmd/outbox-worker

run: ## Run the project locally
	@echo "Running TxStream..."
	go run ./cmd/txstream/main.go

run-worker: ## Run the outbox worker
	@echo "Running Outbox Worker..."
	go run ./cmd/outbox-worker/main.go

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v ./tests/integration/...

test-unit: ## Run unit tests
	@echo "Running unit tests..."
	go test -v ./tests/unit/...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

migrate: ## Run database migrations
	@echo "Running database migrations..."
	go run ./cmd/migrate/main.go

clean: ## Clean build files
	@echo "Cleaning build files..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Docker
docker-build: ## Build the Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Run the Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE)

# Desenvolvimento
dev: ## Run in development mode with hot reload
	@echo "Running in development mode..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not found. Installing..."; \
		go install github.com/cosmtrek/air@latest; \
		air; \
	fi

lint: ## Run the linter
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi

fmt: ## Format the code
	@echo "Formatting code..."
	go fmt ./...

check: fmt lint test ## Run all checks

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
debug: ## Run in debug mode
	@echo "Running in debug mode..."
	dlv debug ./cmd/txstream/main.go

# Health check
health: ## Check the health of the application
	@echo "Checking the health of the application..."
	@curl -f http://localhost:8080/health || echo "Application is not responding"

# Logs
logs: ## Show logs of the application (if running in Docker)
	@echo "Showing logs..."
	@docker logs -f txstream 2>/dev/null || echo "Container txstream not found"


start-all: ## Start all services (Docker + Kafka UI local)
	@echo "Starting all services..."
	@echo "Starting Docker services..."
	@docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "starting Kafka UI locally..."
	@make kafka-ui-start
	@echo "URLs:"
	@echo "   Kafka UI: http://localhost:8082"
	@echo "   Prometheus: http://localhost:9090"
	@echo "   Grafana: http://localhost:3000"
	@echo "   API: http://localhost:8080"
	@echo "To start the worker: make run-worker"
	@echo "To stop all services: make stop-all"

stop-all: ## Stop all services
	@echo "Stopping all services..."
	@docker-compose down
	@make kafka-ui-stop
	@echo "All services stopped"

# Kafka UI Local
kafka-ui-download: ## Download Kafka UI JAR file
	@echo "Downloading Kafka UI..."
	@if [ ! -f kafka-ui.jar ]; then \
		curl -L -o kafka-ui.jar https://github.com/provectus/kafka-ui/releases/download/v0.7.1/kafka-ui-api-v0.7.1.jar; \
		echo "Kafka UI downloaded successfully"; \
	else \
		echo "Kafka UI JAR already exists"; \
	fi

kafka-ui-start: kafka-ui-download ## Start Kafka UI locally
	@echo "Starting Kafka UI locally..."
	@echo "Kafka UI will be available at: http://localhost:8082"
	@echo "Press Ctrl+C to stop"
	@java -Dspring.config.additional-location=./monitoring/kafka-ui-config.yml -Dserver.port=8082 -jar kafka-ui.jar

kafka-ui-stop: ## Stop Kafka UI (if running)
	@echo "Stopping Kafka UI..."
	@pkill -f "kafka-ui.jar" || echo "Kafka UI not running"

kafka-ui-clean: ## Remove Kafka UI JAR file
	@echo "🧹 Cleaning Kafka UI files..."
	@rm -f kafka-ui.jar
	@echo "Kafka UI files cleaned"

# Default target
.DEFAULT_GOAL := help 