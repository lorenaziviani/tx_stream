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

run: ## Run the project locally
	@echo "Running TxStream..."
	go run ./cmd/txstream/main.go

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

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

# Default target
.DEFAULT_GOAL := help 