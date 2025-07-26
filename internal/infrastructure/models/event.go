package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Event struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AggregateID   string         `gorm:"not null;index" json:"aggregate_id"`
	AggregateType string         `gorm:"type:varchar(100);not null;index" json:"aggregate_type"`
	EventType     string         `gorm:"type:varchar(100);not null" json:"event_type"`
	EventData     EventJSON      `gorm:"type:jsonb;not null" json:"event_data"`
	EventMetadata EventJSON      `gorm:"type:jsonb" json:"event_metadata,omitempty"`
	Version       int            `gorm:"not null;default:1" json:"version"`
	CreatedAt     time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP;index" json:"created_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

type EventJSON map[string]interface{}

// Value implements the driver.Valuer interface
func (ej EventJSON) Value() (interface{}, error) {
	if ej == nil {
		return nil, nil
	}
	return ej, nil
}

// Scan implements the sql.Scanner interface
func (ej *EventJSON) Scan(value interface{}) error {
	if value == nil {
		*ej = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, ej)
	case string:
		return json.Unmarshal([]byte(v), ej)
	default:
		return fmt.Errorf("cannot scan %T into EventJSON", value)
	}
}

// Event Types
const (
	EventTypeOrderCreated   = "OrderCreated"
	EventTypeOrderConfirmed = "OrderConfirmed"
	EventTypeOrderShipped   = "OrderShipped"
	EventTypeOrderDelivered = "OrderDelivered"
	EventTypeOrderCancelled = "OrderCancelled"
)

// Aggregate Types
const (
	AggregateTypeOrder = "Order"
	AggregateTypeUser  = "User"
)

func (Event) TableName() string {
	return "events"
}

// BeforeCreate hook to generate UUID if not provided
func (e *Event) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	if err := e.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate validates the Event
func (e *Event) Validate() error {
	if e.AggregateID == "" {
		return fmt.Errorf("aggregate_id is required")
	}
	if e.AggregateType == "" {
		return fmt.Errorf("aggregate_type is required")
	}
	if e.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if e.EventData == nil {
		return fmt.Errorf("event_data is required")
	}
	return nil
}

// NewEvent creates a new event
func NewEvent(aggregateID, aggregateType, eventType string, eventData, eventMetadata map[string]interface{}) *Event {
	return &Event{
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		EventData:     EventJSON(eventData),
		EventMetadata: EventJSON(eventMetadata),
		Version:       1,
	}
}

// GetEventData returns the event data as a map
func (e *Event) GetEventData() map[string]interface{} {
	if e.EventData == nil {
		return make(map[string]interface{})
	}
	return map[string]interface{}(e.EventData)
}

// GetEventMetadata returns the event metadata as a map
func (e *Event) GetEventMetadata() map[string]interface{} {
	if e.EventMetadata == nil {
		return make(map[string]interface{})
	}
	return map[string]interface{}(e.EventMetadata)
}

// SetEventData defines the event data
func (e *Event) SetEventData(data map[string]interface{}) {
	e.EventData = EventJSON(data)
}

// SetEventMetadata defines the event metadata
func (e *Event) SetEventMetadata(metadata map[string]interface{}) {
	e.EventMetadata = EventJSON(metadata)
}

// AddMetadata adds metadata to the event
func (e *Event) AddMetadata(key string, value interface{}) {
	if e.EventMetadata == nil {
		e.EventMetadata = make(EventJSON)
	}
	e.EventMetadata[key] = value
}
