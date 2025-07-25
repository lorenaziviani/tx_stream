-- Migration 002: Create orders table
-- Example table to demonstrate the Outbox pattern

CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id VARCHAR(255) NOT NULL,
    order_number VARCHAR(50) UNIQUE NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    total_amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'BRL',
    items JSONB NOT NULL,
    shipping_address JSONB,
    billing_address JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Indexes for performance
    INDEX idx_orders_customer_id (customer_id),
    INDEX idx_orders_status (status),
    INDEX idx_orders_created_at (created_at),
    INDEX idx_orders_order_number (order_number)
);

-- Trigger to update updated_at automatically
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_orders_updated_at 
    BEFORE UPDATE ON orders 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Comments for documentation
COMMENT ON TABLE orders IS 'Orders table - example of aggregate to demonstrate the Outbox pattern';
COMMENT ON COLUMN orders.id IS 'Unique identifier of the order';
COMMENT ON COLUMN orders.customer_id IS 'ID of the customer that made the order';
COMMENT ON COLUMN orders.order_number IS 'Unique order number';
COMMENT ON COLUMN orders.status IS 'Order status: pending, confirmed, shipped, delivered, cancelled';
COMMENT ON COLUMN orders.total_amount IS 'Total amount of the order';
COMMENT ON COLUMN orders.currency IS 'Currency of the order';
COMMENT ON COLUMN orders.items IS 'List of items in the order in JSON format';
COMMENT ON COLUMN orders.shipping_address IS 'Shipping address in JSON format';
COMMENT ON COLUMN orders.billing_address IS 'Billing address in JSON format';
COMMENT ON COLUMN orders.created_at IS 'Order creation date';
COMMENT ON COLUMN orders.updated_at IS 'Last update date of the order'; 