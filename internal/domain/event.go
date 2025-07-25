package domain

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID            uuid.UUID              `json:"id"`
	AggregateID   string                 `json:"aggregate_id"`
	AggregateType string                 `json:"aggregate_type"`
	EventType     string                 `json:"event_type"`
	EventData     map[string]interface{} `json:"event_data"`
	EventMetadata map[string]interface{} `json:"event_metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// NewEvent creates a new instance of Event
func NewEvent(aggregateID, aggregateType, eventType string, eventData map[string]interface{}) *Event {
	return &Event{
		ID:            uuid.New(),
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		EventData:     eventData,
		EventMetadata: make(map[string]interface{}),
		CreatedAt:     time.Now(),
	}
}

// AddMetadata adds metadata to the event
func (e *Event) AddMetadata(key string, value interface{}) {
	if e.EventMetadata == nil {
		e.EventMetadata = make(map[string]interface{})
	}
	e.EventMetadata[key] = value
}

// EventTypes for Order events
const (
	EventTypeOrderCreated   = "OrderCreated"
	EventTypeOrderConfirmed = "OrderConfirmed"
	EventTypeOrderShipped   = "OrderShipped"
	EventTypeOrderDelivered = "OrderDelivered"
	EventTypeOrderCancelled = "OrderCancelled"
)

// AggregateTypes for aggregates
const (
	AggregateTypeOrder = "Order"
	AggregateTypeUser  = "User"
)
