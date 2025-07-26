# TxStream – Sistema Transacional com Outbox e Kafka

## 📘 Descrição

O TxStream é um sistema que garante consistência ACID ao criar entidades e eventos numa mesma transação (Outbox Pattern), e publica esses eventos num cluster Kafka de forma assíncrona, resiliente e idempotente.

## 🏗️ Arquitetura

O projeto utiliza uma arquitetura hexagonal (Clean Architecture) com as seguintes camadas:

- **Domain**: Entidades e regras de negócio
- **Application**: Casos de uso e serviços de aplicação
- **Infrastructure**: Implementações concretas (banco de dados, Kafka, etc.)

### Outbox Pattern

O sistema implementa o padrão Outbox para garantir:

- **Consistência ACID**: Transações atômicas entre entidades e eventos
- **Resiliência**: Eventos não são perdidos mesmo em caso de falha
- **Idempotência**: Processamento seguro de eventos duplicados

## 🚀 Como Executar

### Pré-requisitos

- Go 1.21+
- PostgreSQL
- Apache Kafka (com KRaft - sem Zookeeper)
- Docker (opcional)

### Configuração

1. Clone o repositório:

```bash
git clone https://github.com/lorenaziviani/txstream.git
cd txstream
```

2. Instale as dependências:

```bash
go mod download
```

3. Configure as variáveis de ambiente:

```bash
cp env.example .env
# Edite o arquivo .env com suas configurações
```

**Configurações Disponíveis:**

- **Server**: Host, porta, timeouts
- **Database**: Conexão, pool de conexões, configurações GORM
- **Kafka**: Brokers, tópicos, configurações de producer/consumer
- **Worker**: Intervalo de polling, tamanho do lote, retry
- **Logging**: Nível, formato, output

4. Inicie o servidor (as migrações são executadas automaticamente):

```bash
go run cmd/txstream/main.go
```

### 🚀 Outbox Worker

O Outbox Worker processa eventos pendentes do outbox e os publica no Kafka:

```bash
# Executar o worker
make run-worker

# Ou compilar e executar
make build-worker
./build/outbox-worker
```

**Funcionalidades do Worker:**

- 🔄 **Polling automático**: Verifica eventos pendentes a cada 5 segundos
- 📦 **Processamento em lote**: Processa até 10 eventos por vez
- 🚀 **Worker Pool**: Processamento paralelo com pool configurável de workers
- 📋 **Log detalhado**: Exibe informações completas dos eventos
- 🛑 **Graceful shutdown**: Para corretamente com Ctrl+C
- 📨 **Publicação Kafka**: Publica eventos no tópico `txstream.events`
- 🔄 **Retry automático**: Reintenta eventos falhados até 3 vezes
- ✅ **Status tracking**: Marca eventos como `published` ou `failed`
- 🔒 **Idempotência**: Garante que eventos não sejam processados duplicadamente

### 🧪 Testando a API

Após iniciar o servidor, você pode testar os endpoints:

#### Criar um Pedido (Transação ACID)

```bash
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "customer-123",
    "order_number": "ORD-001",
    "items": [
      {
        "product_id": "prod-1",
        "product_name": "Produto 1",
        "quantity": 2,
        "unit_price": 75.00
      }
    ],
    "shipping_address": {
      "street": "Rua das Flores",
      "number": "123",
      "city": "São Paulo",
      "state": "SP",
      "zip_code": "01234-567",
      "country": "Brasil"
    },
    "billing_address": {
      "street": "Rua das Flores",
      "number": "123",
      "city": "São Paulo",
      "state": "SP",
      "zip_code": "01234-567",
      "country": "Brasil"
    }
  }'
```

#### Health Check

```bash
curl http://localhost:8080/health
```

#### Listar Pedidos

```bash
curl http://localhost:8080/api/v1/orders
```

## 🧪 Testes

### **Testes de Integração**

Os testes de integração validam a transação ACID e o padrão Outbox:

```bash
# Executar testes de integração
make test-integration

# Executar teste específico
go test -v ./tests/integration/ -run TestOrderTransactionWithOutbox
```

### **Cenários Testados**

- ✅ **Transação bem-sucedida**: Order + OutboxEvent criados
- ✅ **Falha no outbox**: Rollback completo da transação
- ✅ **Pedido duplicado**: Retorna conflito (409)
- ✅ **Request inválido**: Validação antes da transação
- ✅ **Transações concorrentes**: Isolamento garantido
- ✅ **Integridade de dados**: Dados consistentes entre Order e Event

### **Todos os Testes**

```bash
# Executar todos os testes
make test

# Executar testes com cobertura
make test-coverage

# Executar apenas testes unitários
make test-unit
```

### **Documentação Detalhada**

Veja [docs/testing.md](docs/testing.md) para detalhes completos sobre a estratégia de testes.

## 📁 Estrutura do Projeto

```
txstream/
├── cmd/
│   ├── txstream/          # Aplicação principal
│   ├── outbox-worker/     # Worker para processar eventos do outbox
│   └── migrate/           # Ferramenta de migração (legado)
├── internal/
│   ├── application/       # Casos de uso
│   │   ├── dto/           # Data Transfer Objects
│   │   └── usecases/      # Casos de uso da aplicação
│   └── infrastructure/    # Implementações externas
│       ├── models/        # Modelos GORM + Lógica de Domínio
│       ├── repositories/  # Repositórios
│       ├── handlers/      # Handlers HTTP
│       └── database/      # Configuração do banco
├── migrations/            # Migrações SQL (legado)
├── docs/                  # Documentação e diagramas
└── tests/                 # Testes de integração
```

## 🔧 Tecnologias

- **Linguagem**: Go 1.21+
- **Banco de Dados**: PostgreSQL
- **ORM**: GORM
- **Message Broker**: Apache Kafka (KRaft)
- **Kafka Client**: Sarama
- **HTTP Router**: Gorilla Mux
- **Configuração**: Viper
- **Testes**: Testify
- **Mocks**: Mockery v3
- **Concorrência**: sync.WaitGroup, channels, goroutines

## 📊 Diagramas

Consulte a pasta `docs/` para diagramas da arquitetura e fluxos do sistema.

## 🤝 Contribuição

1. Fork o projeto
2. Crie uma branch para sua feature (`git checkout -b feature/AmazingFeature`)
3. Commit suas mudanças (`git commit -m 'Add some AmazingFeature'`)
4. Push para a branch (`git push origin feature/AmazingFeature`)
5. Abra um Pull Request

## 📄 Licença

Este projeto está sob a licença MIT. Veja o arquivo `LICENSE` para mais detalhes.

## 👥 Autores

- **Lorena Ziviani** - _Desenvolvimento inicial_ - [lorenaziviani](https://github.com/lorenaziviani)
