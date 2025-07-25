-- Migration 001: Create outbox table
-- This table implements the Outbox pattern to ensure ACID consistency

CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB NOT NULL,
    event_metadata JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE,
    retry_count INTEGER DEFAULT 0,
    error_message TEXT,
    
    -- Indexes for performance
    INDEX idx_outbox_status (status),
    INDEX idx_outbox_created_at (created_at),
    INDEX idx_outbox_aggregate (aggregate_id, aggregate_type)
);

-- Comments for documentation
COMMENT ON TABLE outbox IS 'Table to implement the Outbox pattern - stores events pending publication';
COMMENT ON COLUMN outbox.id IS 'Unique identifier of the event';
COMMENT ON COLUMN outbox.aggregate_id IS 'ID of the aggregate that generated the event';
COMMENT ON COLUMN outbox.aggregate_type IS 'Aggregate type (ex: Order, User, etc.)';
COMMENT ON COLUMN outbox.event_type IS 'Event type (ex: OrderCreated, UserUpdated, etc.)';
COMMENT ON COLUMN outbox.event_data IS 'Event data in JSON format';
COMMENT ON COLUMN outbox.event_metadata IS 'Additional event metadata';
COMMENT ON COLUMN outbox.status IS 'Event status: pending, published, failed';
COMMENT ON COLUMN outbox.created_at IS 'Event creation date';
COMMENT ON COLUMN outbox.published_at IS 'Event publication date in Kafka';
COMMENT ON COLUMN outbox.retry_count IS 'Number of publication attempts';
COMMENT ON COLUMN outbox.error_message IS 'Error message in case of failure'; 