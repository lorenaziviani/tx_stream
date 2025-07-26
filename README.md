# TX Stream - Event Streaming Platform

Uma plataforma robusta de streaming de eventos construída em Go, implementando o padrão Outbox para garantir consistência de dados e resiliência.

## 🚀 Funcionalidades

### Core Features

- **Outbox Pattern**: Garantia de consistência ACID entre transações de negócio e publicação de eventos
- **Worker Pool**: Processamento paralelo de eventos com pool limitado
- **Circuit Breaker**: Proteção contra falhas em cascata no Kafka
- **Retry Exponencial**: Backoff inteligente com jitter para retry de eventos
- **Métricas Prometheus**: Monitoramento completo do sistema

### Funcionalidades do Worker

- **Processamento Paralelo**: Worker pool com `sync.WaitGroup` e channels
- **Race Condition Protection**: `SELECT FOR UPDATE` para evitar condições de corrida
- **Circuit Breaker**: Estados CLOSED, OPEN, HALF-OPEN com transições automáticas
- **Retry Exponencial**: Backoff exponencial com jitter para retry inteligente, evitando sobrecarga no Kafka durante problemas temporários
- **Métricas em Tempo Real**: Contadores, histogramas e gauges para monitoramento

## 🛠️ Tecnologias

- **Backend**: Go 1.21+
- **Database**: PostgreSQL com GORM
- **Message Broker**: Apache Kafka (KRaft)
- **HTTP Router**: Gorilla Mux
- **Configuration**: Viper
- **Testing**: Testify + Mockery v3
- **Concurrency**: Go routines, sync.WaitGroup, channels
- **Resiliência**: Circuit Breaker pattern
- **Retry Inteligente**: Backoff exponencial com jitter para evitar thundering herd
- **Monitoramento**: Prometheus metrics

## 📊 Métricas do Prometheus

O sistema expõe métricas detalhadas via Prometheus no endpoint `/metrics` (porta 9090).

### Counters

- `txstream_events_processed_total` - Total de eventos processados (por status e tipo)
- `txstream_events_published_total` - Total de eventos publicados no Kafka (por tópico e tipo)
- `txstream_events_failed_total` - Total de eventos que falharam (por tipo de erro e tipo)
- `txstream_events_retried_total` - Total de eventos retry (por tentativa e tipo)
- `txstream_circuit_breaker_trips_total` - Mudanças de estado do Circuit Breaker

### Histograms

- `txstream_event_processing_duration_seconds` - Tempo de processamento de eventos
- `txstream_event_publishing_duration_seconds` - Tempo de publicação no Kafka
- `txstream_retry_delay_duration_seconds` - Duração dos delays de retry

### Gauges

- `txstream_worker_pool_size` - Tamanho atual do worker pool
- `txstream_events_in_queue` - Eventos na fila (por status)
- `txstream_circuit_breaker_state` - Estado atual do Circuit Breaker (0=Closed, 1=Half-Open, 2=Open)
- `txstream_active_workers` - Número de workers ativos

### Exemplo de Query Prometheus

```promql
# Taxa de eventos processados por minuto
rate(txstream_events_processed_total[1m])

# Latência média de publicação
histogram_quantile(0.95, rate(txstream_event_publishing_duration_seconds_bucket[5m]))

# Estado do Circuit Breaker
txstream_circuit_breaker_state

# Eventos na fila
txstream_events_in_queue
```

## ⚙️ Configuração

### Variáveis de Ambiente

#### Configuração Geral

```bash
APP_NAME=txstream
APP_VERSION=1.0.0
APP_ENV=development
```

#### Servidor

```bash
SERVER_PORT=8080
SERVER_HOST=localhost
SERVER_TIMEOUT=30s
```

#### Database

```bash
DB_HOST=localhost
DB_PORT=5432
DB_NAME=txstream
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSL_MODE=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5m
DB_LOG_LEVEL=info
```

#### Kafka

```bash
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC_EVENTS=txstream.events
KAFKA_GROUP_ID=txstream-consumer-group
KAFKA_REQUIRED_ACKS=1
KAFKA_TIMEOUT=30s
KAFKA_AUTO_OFFSET_RESET=earliest
KAFKA_SESSION_TIMEOUT=30s
KAFKA_MAX_RETRIES=3
KAFKA_RETRY_DELAY=1s
```

#### Circuit Breaker

```bash
KAFKA_CIRCUIT_BREAKER_ENABLED=false
KAFKA_FAILURE_THRESHOLD=5
KAFKA_SUCCESS_THRESHOLD=3
KAFKA_TIMEOUT_DURATION=10s
KAFKA_RESET_TIMEOUT=30s
```

#### Retry Exponencial

```bash
KAFKA_EXPONENTIAL_RETRY_ENABLED=false
KAFKA_BASE_DELAY=1s
KAFKA_MAX_DELAY=30s
KAFKA_MULTIPLIER=2.0
```

#### Worker

```bash
WORKER_POOL_SIZE=3
WORKER_BATCH_SIZE=10
WORKER_INTERVAL=5s
WORKER_MAX_RETRIES=3
WORKER_RETRY_DELAY=1s
```

#### Métricas

```bash
METRICS_ENABLED=true
METRICS_PORT=9090
METRICS_PATH=/metrics
```

#### Logging

```bash
LOGGING_LEVEL=info
LOGGING_FORMAT=json
```

## 🏗️ Arquitetura

### Diagrama de Arquitetura

![Arquitetura TX Stream](docs/architecture.drawio)

### Componentes Principais

1. **API Gateway**: Recebe requisições HTTP e roteia para use cases
2. **Application Layer**: Contém a lógica de negócio (use cases)
3. **Domain Layer**: Entidades e regras de domínio
4. **Infrastructure Layer**: Implementações concretas (database, Kafka, etc.)
5. **Outbox Worker**: Processa eventos pendentes e publica no Kafka
6. **Worker Pool**: Gerencia workers paralelos para processamento
7. **Circuit Breaker**: Protege contra falhas em cascata
8. **Kafka Producer**: Publica eventos no tópico configurado

### Fluxo de Dados

1. **HTTP Request** → API Gateway
2. **Use Case** → Domain Layer
3. **Event Creation** → Infrastructure Layer (Database + Outbox)
4. **Background Processing** → Outbox Worker
5. **Event Publishing** → Kafka Producer (com Circuit Breaker)
6. **Event Consumption** → Microservices

## 🚀 Execução

### Pré-requisitos

- Go 1.21+
- PostgreSQL
- Apache Kafka
- Docker (opcional)

### Setup Local

1. **Clone o repositório**

```bash
git clone <repository-url>
cd txstream
```

2. **Configure as variáveis de ambiente**

```bash
cp env.example .env
# Edite o arquivo .env com suas configurações
```

3. **Execute as migrações**

```bash
make migrate
```

4. **Inicie os serviços**

```bash
# Com Docker
docker-compose up -d

# Ou manualmente
# PostgreSQL e Kafka devem estar rodando
```

5. **Execute a aplicação**

```bash
# API Server
make run-api

# Outbox Worker
make run-worker
```

### Comandos Make

```bash
# Build
make build

# Testes
make test
make test-integration

# Migrações
make migrate
make migrate-down

# Execução
make run-api
make run-worker

# Docker
make docker-build
make docker-run

# Limpeza
make clean
```

## 🧪 Testes

### Estrutura de Testes

```
tests/
├── unit/                    # Testes unitários
│   ├── circuit_breaker_test.go
│   ├── exponential_retry_test.go
│   └── ...
├── integration/             # Testes de integração
│   ├── order_transaction_test.go
│   ├── outbox_failure_test.go
│   ├── worker_pool_test.go
│   ├── race_condition_test.go
│   └── ...
└── test_config.go          # Configuração de testes
```

### Executando Testes

```bash
# Todos os testes
make test

# Apenas unitários
go test ./tests/unit/...

# Apenas integração
go test ./tests/integration/...

# Com coverage
go test -cover ./...

# Com verbose
go test -v ./...
```

## �� Monitoramento

### Prometheus e Grafana

O projeto inclui configuração completa de monitoramento com Prometheus e Grafana:

```bash
# Iniciar todos os serviços incluindo monitoramento
docker-compose up -d

# Acessar Prometheus
http://localhost:9090

# Acessar Grafana
http://localhost:3000
# Usuário: admin
# Senha: admin
```

### Dashboards Disponíveis

- **TxStream Metrics Dashboard**: Métricas completas da aplicação
  - Eventos processados, publicados e falhados
  - Estado do Circuit Breaker
  - Latência de processamento
  - Tamanho do worker pool
  - Eventos na fila

### Métricas em Tempo Real

As métricas são coletadas automaticamente e podem ser visualizadas em:

1. **Prometheus**: http://localhost:9090

   - Queries personalizadas
   - Alertas configuráveis
   - Histórico de métricas

2. **Grafana**: http://localhost:3000
   - Dashboards pré-configurados
   - Visualizações interativas
   - Alertas e notificações

### Exemplo de Queries Prometheus

```promql
# Taxa de eventos processados por minuto
rate(txstream_events_processed_total[1m])

# Latência média de publicação (95º percentil)
histogram_quantile(0.95, rate(txstream_event_publishing_duration_seconds_bucket[5m]))

# Estado atual do Circuit Breaker
txstream_circuit_breaker_state

# Eventos na fila por status
txstream_events_in_queue

# Taxa de falhas
rate(txstream_events_failed_total[5m])
```

## 📁 Estrutura do Projeto

```
txstream/
├── cmd/                    # Entry points
│   ├── migrate/           # Database migrations
│   └── txstream/          # Main application
├── internal/              # Código interno
│   ├── application/       # Use cases
│   ├── domain/           # Domain entities
│   └── infrastructure/   # External concerns
│       ├── config/       # Configuration
│       ├── database/     # Database connection
│       ├── handlers/     # HTTP handlers
│       ├── kafka/        # Kafka producer
│       ├── metrics/      # Prometheus metrics
│       ├── models/       # Data models
│       ├── repositories/ # Data access
│       └── worker/       # Outbox worker
├── migrations/           # SQL migrations
├── tests/               # Test files
├── docs/               # Documentation
├── docker-compose.yml  # Docker services
├── Dockerfile         # Docker image
├── Makefile          # Build commands
├── go.mod            # Go modules
└── README.md         # This file
```

### Convenções

- **Naming**: camelCase para variáveis, PascalCase para tipos
- **Error Handling**: Sempre retornar e logar erros
- **Logging**: Usar structured logging com contexto
- **Testing**: Cobertura mínima de 80%
- **Documentation**: Comentários em funções públicas

### Contribuindo

1. Fork o projeto
2. Crie uma branch para sua feature
3. Implemente com testes
4. Execute `make test`
5. Commit suas mudanças
6. Push para a branch
7. Abra um Pull Request

## 📄 Licença

Este projeto está sob a licença MIT. Veja o arquivo [LICENSE](LICENSE) para mais detalhes.

## 🤝 Suporte

Para suporte e dúvidas:

- Abra uma [issue](../../issues)
- Consulte a [documentação](docs/)
- Entre em contato com a equipe

---

**TX Stream** - Event Streaming Platform com Outbox Pattern, Circuit Breaker e Métricas Prometheus 🚀
