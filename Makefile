# Makefile para o projeto TxStream

.PHONY: help build run test clean migrate deps docker-build docker-run

# Variáveis
BINARY_NAME=txstream
BUILD_DIR=build
DOCKER_IMAGE=txstream:latest

# Comandos principais
help: ## Mostra esta ajuda
	@echo "Comandos disponíveis:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

deps: ## Instala as dependências do projeto
	go mod download
	go mod tidy

build: ## Compila o projeto
	@echo "🔨 Compilando TxStream..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/txstream

run: ## Executa o projeto localmente
	@echo "🚀 Executando TxStream..."
	go run ./cmd/txstream/main.go

test: ## Executa os testes
	@echo "🧪 Executando testes..."
	go test -v ./...

test-coverage: ## Executa os testes com cobertura
	@echo "🧪 Executando testes com cobertura..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Relatório de cobertura gerado: coverage.html"

migrate: ## Executa as migrações do banco de dados
	@echo "🗄️ Executando migrações..."
	go run ./cmd/migrate/main.go

clean: ## Limpa arquivos de build
	@echo "🧹 Limpando arquivos de build..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Docker
docker-build: ## Constrói a imagem Docker
	@echo "🐳 Construindo imagem Docker..."
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Executa o container Docker
	@echo "🐳 Executando container Docker..."
	docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE)

# Desenvolvimento
dev: ## Executa em modo desenvolvimento com hot reload
	@echo "🔥 Executando em modo desenvolvimento..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "⚠️ Air não encontrado. Instalando..."; \
		go install github.com/cosmtrek/air@latest; \
		air; \
	fi

lint: ## Executa o linter
	@echo "🔍 Executando linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "⚠️ golangci-lint não encontrado. Instalando..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run; \
	fi

fmt: ## Formata o código
	@echo "🎨 Formatando código..."
	go fmt ./...

# Verificações
check: fmt lint test ## Executa todas as verificações

# Instalação de ferramentas
install-tools: ## Instala ferramentas de desenvolvimento
	@echo "🛠️ Instalando ferramentas de desenvolvimento..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
debug: ## Executa em modo debug
	@echo "🐛 Executando em modo debug..."
	dlv debug ./cmd/txstream/main.go

# Health check
health: ## Verifica a saúde da aplicação
	@echo "🏥 Verificando saúde da aplicação..."
	@curl -f http://localhost:8080/health || echo "❌ Aplicação não está respondendo"

# Logs
logs: ## Mostra logs da aplicação (se estiver rodando em Docker)
	@echo "📋 Mostrando logs..."
	@docker logs -f txstream 2>/dev/null || echo "❌ Container txstream não encontrado"

# Default target
.DEFAULT_GOAL := help 