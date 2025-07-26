#!/bin/bash

set -e

echo "Kafka Monitor - Alternative to Kafka UI"
echo "=============================================="
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

monitor_kafka() {
    while true; do
        clear
        echo "Kafka Monitor - Updated at $(date)"
        echo "=============================================="
        echo ""
        
        print_info "Services Status:"
        echo "   Kafka: $(docker-compose ps kafka | grep -o 'Up' || echo 'Down')"
        echo "   Worker: $(ps aux | grep -c 'outbox-worker' || echo '0') processes"
        echo ""
        
        print_info "Events in Outbox:"
        PENDING=$(docker-compose exec -T postgres psql -U txstream_user -d txstream_db -t -c "SELECT COUNT(*) FROM outbox WHERE status = 'pending';" 2>/dev/null | tr -d ' ' || echo "0")
        PUBLISHED=$(docker-compose exec -T postgres psql -U txstream_user -d txstream_db -t -c "SELECT COUNT(*) FROM outbox WHERE status = 'published';" 2>/dev/null | tr -d ' ' || echo "0")
        FAILED=$(docker-compose exec -T postgres psql -U txstream_user -d txstream_db -t -c "SELECT COUNT(*) FROM outbox WHERE status = 'failed';" 2>/dev/null | tr -d ' ' || echo "0")
        
        echo "   Pending: $PENDING"
        echo "   Published: $PUBLISHED"
        echo "   Failed: $FAILED"
        echo ""
        
        print_info "Last 5 Events:"
        docker-compose exec -T postgres psql -U txstream_user -d txstream_db -c "
            SELECT 
                id, 
                aggregate_type, 
                event_type, 
                status, 
                created_at,
                retry_count
            FROM outbox 
            ORDER BY created_at DESC 
            LIMIT 5;" 2>/dev/null || echo "   Erro ao consultar eventos"
        echo ""
        
        print_info "Recent Kafka Logs:"
        docker-compose logs kafka --tail=3 | grep -E "(txstream.events|ERROR|WARN)" || echo "   Nenhum log relevante"
        echo ""
        
        print_info "Worker Metrics:"
        if curl -s http://localhost:9091/metrics >/dev/null 2>&1; then
            EVENTS_PROCESSED=$(curl -s http://localhost:9091/metrics | grep "txstream_events_processed_total" | grep -v "#" | awk '{print $2}' || echo "0")
            EVENTS_FAILED=$(curl -s http://localhost:9091/metrics | grep "txstream_events_failed_total" | grep -v "#" | awk '{print $2}' || echo "0")
            echo "   Eventos Processados: $EVENTS_PROCESSED"
            echo "   Eventos Falharam: $EVENTS_FAILED"
        else
            echo "   Métricas não disponíveis (worker pode não estar rodando)"
        fi
        echo ""
        
        print_info "Useful URLs:"
        echo "   Kafka UI: http://localhost:8081 (may not be working)"
        echo "   Prometheus: http://localhost:9090"
        echo "   Grafana: http://localhost:3000"
        echo "   API: http://localhost:8080"
        echo ""
        
        print_info "Next update in 10 seconds... (Ctrl+C to exit)"
        sleep 10
    done
}

if ! docker-compose ps kafka | grep -q "Up"; then
    print_error "Kafka is not running!"
    exit 1
fi

monitor_kafka 