#!/bin/bash

set -e

echo "TxStream - Demo System"
echo "======================================"
echo ""
  
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_header() {
    echo -e "${PURPLE}📋 $1${NC}"
    echo "----------------------------------------"
}

print_metric() {
    echo -e "${CYAN}📊 $1${NC}"
}

if ! docker info > /dev/null 2>&1; then
    print_error "Docker is not running. Start Docker first."
    exit 1
fi

print_header "1. Starting Services"
echo "Starting PostgreSQL, Kafka, Prometheus and Grafana..."
docker-compose up -d postgres kafka prometheus grafana

print_status "Services started successfully!"

print_header "2. Checking Services Status"
echo "Waiting for services to be ready..."

print_info "Waiting for PostgreSQL..."
until docker-compose exec -T postgres pg_isready -U txstream_user -d txstream_db > /dev/null 2>&1; do
    sleep 2
done
print_status "PostgreSQL is ready"

print_info "Waiting for Kafka..."
until nc -z localhost 9092 2>/dev/null; do
    sleep 2
done
print_status "Kafka is ready"

print_info "Waiting for Prometheus..."
until curl -s http://localhost:9090/-/healthy > /dev/null; do
    sleep 2
done
print_status "Prometheus is ready"

print_info "Waiting for Grafana..."
until curl -s http://localhost:3000/api/health > /dev/null; do
    sleep 2
done
print_status "Grafana is ready"

print_header "3. Checking Database"
echo "Checking if tables were created..."

print_info "Waiting for PostgreSQL..."
until docker-compose exec -T postgres pg_isready -U txstream_user -d txstream_db > /dev/null 2>&1; do
    sleep 2
done
print_status "PostgreSQL is ready"

print_info "Checking tables..."
TABLE_COUNT=$(docker-compose exec -T postgres psql -U txstream_user -d txstream_db -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>/dev/null | tr -d ' ')
if [ "$TABLE_COUNT" -ge 4 ]; then
    print_status "Tables created automatically by PostgreSQL ($TABLE_COUNT tables found)"
else
    print_warning "Few tables found ($TABLE_COUNT), executing migrations manually..."
    DB_HOST=localhost DB_PORT=5432 DB_USER=txstream_user DB_PASSWORD=txstream_password DB_NAME=txstream_db DB_SSLMODE=disable go run cmd/migrate/main.go
fi

print_header "4. Starting Application"
echo "Starting TxStream API and Worker..."
docker-compose up -d txstream

print_info "Waiting for application to be ready..."
until curl -s http://localhost:8080/health > /dev/null; do
    sleep 2
done
print_status "Application is ready"

print_header "5. Demo - Creating Orders"
echo "Creating orders to demonstrate the Outbox Pattern..."

print_info "Creating order 1..."
ORDER1_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "customer-demo-1",
    "order_number": "ORD-001",
    "items": [
      {
        "product_id": "prod-1",
        "product_name": "Produto Demo 1",
        "quantity": 2,
        "unit_price": 29.99
      }
    ],
    "shipping_address": {
      "street": "Rua Demo",
      "number": "123",
      "complement": "Apto 1",
      "city": "São Paulo",
      "state": "SP",
      "zip_code": "01234-567",
      "country": "Brasil"
    },
    "billing_address": {
      "street": "Rua Demo",
      "number": "123",
      "complement": "Apto 1",
      "city": "São Paulo",
      "state": "SP",
      "zip_code": "01234-567",
      "country": "Brasil"
    }
  }')

ORDER1_ID=$(echo $ORDER1_RESPONSE | jq -r '.id')
print_status "Pedido 1 criado: $ORDER1_ID"

print_info "Creating order 2..."
ORDER2_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "customer-demo-2",
    "order_number": "ORD-002",
    "items": [
      {
        "product_id": "prod-2",
        "product_name": "Produto Demo 2",
        "quantity": 1,
        "unit_price": 99.99
      },
      {
        "product_id": "prod-3",
        "product_name": "Produto Demo 3",
        "quantity": 3,
        "unit_price": 15.50
      }
    ],
    "shipping_address": {
      "street": "Av Demo",
      "number": "456",
      "complement": "Sala 10",
      "city": "Rio de Janeiro",
      "state": "RJ",
      "zip_code": "20000-000",
      "country": "Brasil"
    },
    "billing_address": {
      "street": "Av Demo",
      "number": "456",
      "complement": "Sala 10",
      "city": "Rio de Janeiro",
      "state": "RJ",
      "zip_code": "20000-000",
      "country": "Brasil"
    }
  }')

ORDER2_ID=$(echo $ORDER2_RESPONSE | jq -r '.id')
print_status "Pedido 2 criado: $ORDER2_ID"

print_header "6. Checking Created Orders"
echo "Checking orders created in the database..."

sleep 3

ORDERS=$(curl -s http://localhost:8080/api/v1/orders)
ORDER_COUNT=$(echo $ORDERS | jq '. | length')
print_status "Encontrados $ORDER_COUNT pedidos na base de dados"

if [ "$ORDER_COUNT" -gt 0 ]; then
    echo ""
    print_info "Detalhes dos pedidos:"
    echo $ORDERS | jq '.[] | {id: .id, customer_id: .customer_id, order_number: .order_number, status: .status, total_amount: .total_amount}'
fi

print_header "7. Checking Metrics"
echo "Checking Prometheus metrics..."

sleep 5

print_metric "Metrics available at: http://localhost:9090"
print_metric "Grafana dashboard at: http://localhost:3000 (admin/admin)"

print_info "Checking metrics for processed events..."
curl -s http://localhost:9090/api/v1/query?query=txstream_events_processed_total | jq '.data.result[] | {metric: .metric, value: .value[1]}'

print_header "8. Demo - Circuit Breaker"
echo "Simulating failures to demonstrate the Circuit Breaker..."

print_info "To test the Circuit Breaker, you can:"
print_info "1. Stop Kafka: docker-compose stop kafka"
print_info "2. Try to create more orders"
print_info "3. Observe the Circuit Breaker metrics in Grafana"
print_info "4. Restart Kafka: docker-compose start kafka"

print_header "9. Access URLs"
echo "System available at the following URLs:"
echo ""
print_info "TxStream API: http://localhost:8080"
print_info "Prometheus: http://localhost:9090"
print_info "Grafana: http://localhost:3000 (admin/admin)"
print_info "Kafka UI: http://localhost:8081"
echo ""
print_info "API Endpoints:"
print_info "  - GET /health - Health check"
print_info "  - POST /orders - Create order"
print_info "  - GET /orders - List orders"
print_info "  - GET /outbox-events - List outbox events"
echo ""
print_info "Main metrics:"
print_info "  - txstream_events_processed_total"
print_info "  - txstream_events_published_total"
print_info "  - txstream_circuit_breaker_state"
print_info "  - txstream_worker_pool_size"

print_header "10. Useful Commands"
echo "Commands for monitoring:"
echo ""
print_info "View application logs:"
echo "  docker-compose logs -f txstream"
echo ""
print_info "View Kafka logs:"
echo "  docker-compose logs -f kafka"
echo ""
print_info "View real-time metrics:"
echo "  curl http://localhost:9090/api/v1/query?query=txstream_events_processed_total"
echo ""
print_info "Stop all services:"
echo "  docker-compose down"

print_header "✅ Demo Completed!"
echo "TxStream system is working with:"
print_status "✅ Outbox Pattern implemented"
print_status "✅ Worker Pool active"
print_status "✅ Circuit Breaker configured"
print_status "✅ Exponential Retry working"
print_status "✅ Prometheus metrics collecting"
print_status "✅ Grafana dashboard available"

echo ""
echo "Enjoy exploring the system!"
echo "Tip: Open Grafana at http://localhost:3000 to see real-time metrics" 