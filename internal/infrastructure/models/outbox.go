package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OutboxEvent struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AggregateID   string         `gorm:"not null;index" json:"aggregate_id"`
	AggregateType string         `gorm:"type:varchar(100);not null;index" json:"aggregate_type"`
	EventType     string         `gorm:"type:varchar(100);not null" json:"event_type"`
	EventData     JSON           `gorm:"type:jsonb;not null" json:"event_data"`
	EventMetadata JSON           `gorm:"type:jsonb" json:"event_metadata,omitempty"`
	Status        OutboxStatus   `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	CreatedAt     time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP;index" json:"created_at"`
	PublishedAt   *time.Time     `gorm:"index" json:"published_at,omitempty"`
	RetryCount    int            `gorm:"not null;default:0" json:"retry_count"`
	ErrorMessage  string         `gorm:"type:text" json:"error_message,omitempty"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusPublished OutboxStatus = "published"
	OutboxStatusFailed    OutboxStatus = "failed"
)

type JSON map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSON) Value() (interface{}, error) {
	if j == nil {
		return nil, nil
	}
	return j, nil
}

// Scan implements the sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	default:
		return fmt.Errorf("cannot scan %T into JSON", value)
	}
}

func (OutboxEvent) TableName() string {
	return "outbox"
}

// BeforeCreate hook to generate UUID if not provided
func (oe *OutboxEvent) BeforeCreate(tx *gorm.DB) error {
	if oe.ID == uuid.Nil {
		oe.ID = uuid.New()
	}
	if oe.CreatedAt.IsZero() {
		oe.CreatedAt = time.Now()
	}

	if err := oe.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate validates the OutboxEvent
func (oe *OutboxEvent) Validate() error {
	if oe.AggregateID == "" {
		return fmt.Errorf("aggregate_id is required")
	}
	if oe.AggregateType == "" {
		return fmt.Errorf("aggregate_type is required")
	}
	if oe.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if oe.EventData == nil {
		return fmt.Errorf("event_data is required")
	}
	return nil
}

// MarkAsPublished marks the event as published
func (oe *OutboxEvent) MarkAsPublished() {
	now := time.Now()
	oe.Status = OutboxStatusPublished
	oe.PublishedAt = &now
}

// MarkAsFailed marks the event as failed
func (oe *OutboxEvent) MarkAsFailed(errorMsg string) {
	oe.Status = OutboxStatusFailed
	oe.ErrorMessage = errorMsg
	oe.RetryCount++
}

// ResetForRetry resets the event for a new attempt
func (oe *OutboxEvent) ResetForRetry() {
	oe.Status = OutboxStatusPending
	oe.ErrorMessage = ""
}

// IsRetryable checks if the event can be retried
func (oe *OutboxEvent) IsRetryable(maxRetries int) bool {
	return oe.Status == OutboxStatusFailed && oe.RetryCount < maxRetries
}

// GetEventData returns the event data as a map
func (oe *OutboxEvent) GetEventData() map[string]interface{} {
	if oe.EventData == nil {
		return make(map[string]interface{})
	}
	return map[string]interface{}(oe.EventData)
}

// GetEventMetadata returns the event metadata as a map
func (oe *OutboxEvent) GetEventMetadata() map[string]interface{} {
	if oe.EventMetadata == nil {
		return make(map[string]interface{})
	}
	return map[string]interface{}(oe.EventMetadata)
}

// SetEventData defines the event data
func (oe *OutboxEvent) SetEventData(data map[string]interface{}) {
	oe.EventData = JSON(data)
}

// SetEventMetadata defines the event metadata
func (oe *OutboxEvent) SetEventMetadata(metadata map[string]interface{}) {
	oe.EventMetadata = JSON(metadata)
}
