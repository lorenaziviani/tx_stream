e#!/bin/bash

# Demonstração Interativa do Circuit Breaker
# Este script demonstra o Circuit Breaker em diferentes cenários

set -e

echo "🔄 TxStream - Demonstração do Circuit Breaker"
echo "============================================="
echo ""

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

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

# Função para obter estado do Circuit Breaker
get_circuit_breaker_state() {
    local state=$(curl -s "http://localhost:9090/api/v1/query?query=txstream_circuit_breaker_state" | jq -r '.data.result[0].value[1] // "unknown"')
    case $state in
        "0") echo "CLOSED";;
        "1") echo "HALF-OPEN";;
        "2") echo "OPEN";;
        *) echo "UNKNOWN";;
    esac
}

# Função para obter métricas de eventos
get_event_metrics() {
    local processed=$(curl -s "http://localhost:9090/api/v1/query?query=txstream_events_processed_total" | jq -r '.data.result[0].value[1] // "0"')
    local failed=$(curl -s "http://localhost:9090/api/v1/query?query=txstream_events_failed_total" | jq -r '.data.result[0].value[1] // "0"')
    local published=$(curl -s "http://localhost:9090/api/v1/query?query=txstream_events_published_total" | jq -r '.data.result[0].value[1] // "0"')
    echo "Processados: $processed, Publicados: $published, Falhados: $failed"
}

# Verificar se o sistema está rodando
if ! curl -s http://localhost:8080/health > /dev/null; then
    print_error "Sistema não está rodando. Execute primeiro: ./demo/run_demo.sh"
    exit 1
fi

print_header "1. Estado Inicial do Sistema"
print_info "Verificando estado inicial..."

CB_STATE=$(get_circuit_breaker_state)
EVENT_METRICS=$(get_event_metrics)

print_status "Circuit Breaker: $CB_STATE"
print_metric "Eventos: $EVENT_METRICS"

print_header "2. Cenário 1: Sistema Funcionando Normalmente"
print_info "Criando pedidos com sistema funcionando..."

# Criar alguns pedidos
for i in {1..3}; do
    print_info "Criando pedido $i..."
    curl -s -X POST http://localhost:8080/orders \
        -H "Content-Type: application/json" \
        -d "{
            \"customer_id\": \"customer-normal-$i\",
            \"items\": [
                {
                    \"product_id\": \"prod-$i\",
                    \"quantity\": $i,
                    \"unit_price\": $((10 + $i * 5))
                }
            ],
            \"shipping_address\": {
                \"street\": \"Rua Normal\",
                \"number\": \"$i\",
                \"city\": \"São Paulo\",
                \"state\": \"SP\",
                \"zip_code\": \"01234-56$i\",
                \"country\": \"Brasil\"
            }
        }" > /dev/null
    
    sleep 2
done

print_status "Pedidos criados com sucesso"
sleep 3

CB_STATE=$(get_circuit_breaker_state)
EVENT_METRICS=$(get_event_metrics)

print_status "Circuit Breaker: $CB_STATE"
print_metric "Eventos: $EVENT_METRICS"

print_header "3. Cenário 2: Simulando Falha (Parando Kafka)"
print_warning "Parando o Kafka para simular falha..."
docker-compose stop kafka

print_info "Aguardando Kafka parar..."
sleep 5

print_info "Tentando criar pedidos (vai falhar)..."
for i in {1..5}; do
    print_info "Tentativa $i..."
    curl -s -X POST http://localhost:8080/orders \
        -H "Content-Type: application/json" \
        -d "{
            \"customer_id\": \"customer-fail-$i\",
            \"items\": [
                {
                    \"product_id\": \"prod-fail-$i\",
                    \"quantity\": 1,
                    \"unit_price\": 10
                }
            ],
            \"shipping_address\": {
                \"street\": \"Rua Falha\",
                \"number\": \"$i\",
                \"city\": \"São Paulo\",
                \"state\": \"SP\",
                \"zip_code\": \"01234-56$i\",
                \"country\": \"Brasil\"
            }
        }" > /dev/null
    
    sleep 2
done

print_warning "Pedidos falharam (esperado)"
sleep 3

CB_STATE=$(get_circuit_breaker_state)
EVENT_METRICS=$(get_event_metrics)

print_status "Circuit Breaker: $CB_STATE"
print_metric "Eventos: $EVENT_METRICS"

if [ "$CB_STATE" = "OPEN" ]; then
    print_status "✅ Circuit Breaker abriu corretamente!"
else
    print_warning "⚠️  Circuit Breaker ainda não abriu (pode precisar de mais falhas)"
fi

print_header "4. Cenário 3: Recuperação (Reiniciando Kafka)"
print_info "Reiniciando o Kafka..."
docker-compose start kafka

print_info "Aguardando Kafka ficar pronto..."
sleep 10

# Verificar se Kafka está pronto
until nc -z localhost 9092 2>/dev/null; do
    sleep 2
done

print_status "Kafka está pronto novamente"

print_info "Aguardando Circuit Breaker tentar recuperação..."
sleep 15

CB_STATE=$(get_circuit_breaker_state)
EVENT_METRICS=$(get_event_metrics)

print_status "Circuit Breaker: $CB_STATE"
print_metric "Eventos: $EVENT_METRICS"

if [ "$CB_STATE" = "CLOSED" ]; then
    print_status "✅ Circuit Breaker fechou corretamente após recuperação!"
elif [ "$CB_STATE" = "HALF-OPEN" ]; then
    print_info "🔄 Circuit Breaker em HALF-OPEN (testando recuperação)"
else
    print_warning "⚠️  Circuit Breaker ainda em $CB_STATE"
fi

print_header "5. Cenário 4: Testando Recuperação Completa"
print_info "Criando pedidos para testar recuperação completa..."

for i in {1..3}; do
    print_info "Criando pedido de recuperação $i..."
    curl -s -X POST http://localhost:8080/orders \
        -H "Content-Type: application/json" \
        -d "{
            \"customer_id\": \"customer-recovery-$i\",
            \"items\": [
                {
                    \"product_id\": \"prod-recovery-$i\",
                    \"quantity\": $i,
                    \"unit_price\": $((20 + $i * 10))
                }
            ],
            \"shipping_address\": {
                \"street\": \"Rua Recuperação\",
                \"number\": \"$i\",
                \"city\": \"São Paulo\",
                \"state\": \"SP\",
                \"zip_code\": \"01234-56$i\",
                \"country\": \"Brasil\"
            }
        }" > /dev/null
    
    sleep 2
done

print_status "Pedidos de recuperação criados"
sleep 5

CB_STATE=$(get_circuit_breaker_state)
EVENT_METRICS=$(get_event_metrics)

print_status "Circuit Breaker: $CB_STATE"
print_metric "Eventos: $EVENT_METRICS"

print_header "6. Resumo da Demonstração"
echo "Demonstração do Circuit Breaker concluída!"
echo ""
print_info "Estados observados:"
print_info "  - CLOSED: Sistema funcionando normalmente"
print_info "  - OPEN: Sistema protegido contra falhas"
print_info "  - HALF-OPEN: Sistema testando recuperação"
echo ""
print_info "Métricas finais:"
print_metric "Circuit Breaker: $CB_STATE"
print_metric "Eventos: $EVENT_METRICS"
echo ""
print_info "URLs para monitoramento:"
print_info "  - Grafana: http://localhost:3000 (admin/admin)"
print_info "  - Prometheus: http://localhost:9090"
print_info "  - API: http://localhost:8080/health"
echo ""
print_status "✅ Demonstração do Circuit Breaker concluída com sucesso!" 