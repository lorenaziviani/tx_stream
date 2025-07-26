#!/bin/bash

# Script to setup test database
echo "Setting up test database..."

# Check if container is running
if ! docker ps | grep -q txstream-postgres; then
    echo "Container txstream-postgres is not running. Execute 'docker-compose up -d' first."
    exit 1
fi

# Create test database if it doesn't exist
echo "Creating test database txstream_test..."
docker exec -it txstream-postgres psql -U txstream_user -d txstream_db -c "SELECT 1 FROM pg_database WHERE datname='txstream_test';" | grep -q 1 || \
docker exec -it txstream-postgres psql -U txstream_user -d txstream_db -c "CREATE DATABASE txstream_test;"

# Execute migrations
echo "Executando migrações no banco de teste..."
docker exec -it txstream-postgres psql -U txstream_user -d txstream_test -c "
-- Cria tabela orders se não existir
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id VARCHAR(255) NOT NULL,
    order_number VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    total_amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'BRL',
    shipping_address_street VARCHAR(255),
    shipping_address_number VARCHAR(50),
    shipping_address_complement VARCHAR(255),
    shipping_address_city VARCHAR(100),
    shipping_address_state VARCHAR(50),
    shipping_address_zip_code VARCHAR(20),
    shipping_address_country VARCHAR(100),
    billing_address_street VARCHAR(255),
    billing_address_number VARCHAR(50),
    billing_address_complement VARCHAR(255),
    billing_address_city VARCHAR(100),
    billing_address_state VARCHAR(50),
    billing_address_zip_code VARCHAR(20),
    billing_address_country VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create order_items table if it doesn't exist
CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id VARCHAR(255) NOT NULL,
    product_name VARCHAR(255) NOT NULL,
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    total_price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create outbox table if it doesn't exist
CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB NOT NULL,
    event_metadata JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP,
    retry_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    deleted_at TIMESTAMP
);

-- Create events table if it doesn't exist
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB NOT NULL,
    event_metadata JSONB,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_order_number ON orders(order_number);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_deleted_at ON orders(deleted_at);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate_id ON outbox(aggregate_id);
CREATE INDEX IF NOT EXISTS idx_outbox_aggregate_type ON outbox(aggregate_type);
CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox(status);
CREATE INDEX IF NOT EXISTS idx_outbox_created_at ON outbox(created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_deleted_at ON outbox(deleted_at);
CREATE INDEX IF NOT EXISTS idx_events_aggregate_id ON events(aggregate_id);
CREATE INDEX IF NOT EXISTS idx_events_aggregate_type ON events(aggregate_type);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_deleted_at ON events(deleted_at);
"

echo "Test database setup completed successfully!"
echo "Now you can run: make test-integration" 