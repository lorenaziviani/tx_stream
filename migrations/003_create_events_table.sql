-- Migration 003: Create events table
-- Table to store the history of processed events

CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outbox_id UUID REFERENCES outbox(id),
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB NOT NULL,
    event_metadata JSONB,
    kafka_topic VARCHAR(255),
    kafka_partition INTEGER,
    kafka_offset BIGINT,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Indexes for performance
    INDEX idx_events_aggregate (aggregate_id, aggregate_type),
    INDEX idx_events_type (event_type),
    INDEX idx_events_processed_at (processed_at),
    INDEX idx_events_outbox_id (outbox_id)
);

-- Comments for documentation
COMMENT ON TABLE events IS 'Table to store the history of processed events';
COMMENT ON COLUMN events.id IS 'Unique identifier of the processed event';
COMMENT ON COLUMN events.outbox_id IS 'Reference to the outbox table (optional)';
COMMENT ON COLUMN events.aggregate_id IS 'ID of the aggregate that generated the event';
COMMENT ON COLUMN events.aggregate_type IS 'Aggregate type';
COMMENT ON COLUMN events.event_type IS 'Event type';
COMMENT ON COLUMN events.event_data IS 'Event data in JSON format';
COMMENT ON COLUMN events.event_metadata IS 'Additional event metadata';
COMMENT ON COLUMN events.kafka_topic IS 'Kafka topic where the event was published';
COMMENT ON COLUMN events.kafka_partition IS 'Kafka partition where the event was published';
COMMENT ON COLUMN events.kafka_offset IS 'Kafka offset of the event';
COMMENT ON COLUMN events.processed_at IS 'Event processing date'; 