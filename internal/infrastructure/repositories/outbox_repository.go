package repositories

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

type OutboxRepository interface {
	Create(ctx context.Context, event *models.OutboxEvent) error
	GetByID(ctx context.Context, id string) (*models.OutboxEvent, error)
	GetPendingEvents(ctx context.Context, limit int) ([]models.OutboxEvent, error)
	GetFailedEvents(ctx context.Context, limit int) ([]models.OutboxEvent, error)
	Update(ctx context.Context, event *models.OutboxEvent) error
	Delete(ctx context.Context, id string) error
	MarkAsPublished(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string, errorMsg string) error
	GetEventsByAggregate(ctx context.Context, aggregateID, aggregateType string) ([]models.OutboxEvent, error)
	GetEventsByType(ctx context.Context, eventType string, limit, offset int) ([]models.OutboxEvent, error)
	CleanupOldEvents(ctx context.Context, olderThan time.Duration) error

	GetPendingEventForUpdate(ctx context.Context, id string) (*models.OutboxEvent, error)
	MarkAsPublishedWithLock(ctx context.Context, id string) error
	MarkAsFailedWithLock(ctx context.Context, id string, errorMsg string) error
}

type outboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepository{db: db}
}

// Create creates a new outbox event
func (r *outboxRepository) Create(ctx context.Context, event *models.OutboxEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// GetByID gets an outbox event by ID
func (r *outboxRepository) GetByID(ctx context.Context, id string) (*models.OutboxEvent, error) {
	var event models.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&event).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("outbox event not found with id: %s", id)
		}
		return nil, err
	}

	return &event, nil
}

// GetPendingEventForUpdate gets a pending event with row lock to prevent race conditions
func (r *outboxRepository) GetPendingEventForUpdate(ctx context.Context, id string) (*models.OutboxEvent, error) {
	var event models.OutboxEvent
	err := r.db.WithContext(ctx).
		Raw("SELECT * FROM outbox WHERE id = ? AND status = ? FOR UPDATE", id, models.OutboxStatusPending).
		First(&event).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("pending outbox event not found with id: %s", id)
		}
		return nil, err
	}

	return &event, nil
}

// GetPendingEvents gets pending events for publication
func (r *outboxRepository) GetPendingEvents(ctx context.Context, limit int) ([]models.OutboxEvent, error) {
	var events []models.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("status = ?", models.OutboxStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error

	return events, err
}

// GetFailedEvents gets events that failed publication
func (r *outboxRepository) GetFailedEvents(ctx context.Context, limit int) ([]models.OutboxEvent, error) {
	var events []models.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("status = ?", models.OutboxStatusFailed).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error

	return events, err
}

// Update updates an existing outbox event
func (r *outboxRepository) Update(ctx context.Context, event *models.OutboxEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

// Delete removes an outbox event
func (r *outboxRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.OutboxEvent{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("outbox event not found with id: %s", id)
	}

	return nil
}

// MarkAsPublished marks an event as published
func (r *outboxRepository) MarkAsPublished(ctx context.Context, id string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       models.OutboxStatusPublished,
			"published_at": &now,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("outbox event not found with id: %s", id)
	}

	return nil
}

// MarkAsPublishedWithLock marks an event as published with row lock to prevent race conditions
func (r *outboxRepository) MarkAsPublishedWithLock(ctx context.Context, id string) error {
	event, err := r.GetPendingEventForUpdate(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now()
	event.Status = models.OutboxStatusPublished
	event.PublishedAt = &now

	return r.db.WithContext(ctx).Save(event).Error
}

// MarkAsFailed marks an event as failed
func (r *outboxRepository) MarkAsFailed(ctx context.Context, id string, errorMsg string) error {
	result := r.db.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.OutboxStatusFailed,
			"error_message": errorMsg,
			"retry_count":   gorm.Expr("retry_count + 1"),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("outbox event not found with id: %s", id)
	}

	return nil
}

// MarkAsFailedWithLock marks an event as failed with row lock to prevent race conditions
func (r *outboxRepository) MarkAsFailedWithLock(ctx context.Context, id string, errorMsg string) error {
	event, err := r.GetPendingEventForUpdate(ctx, id)
	if err != nil {
		return err
	}

	event.Status = models.OutboxStatusFailed
	event.ErrorMessage = errorMsg
	event.RetryCount++

	return r.db.WithContext(ctx).Save(event).Error
}

// GetEventsByAggregate gets events by aggregate
func (r *outboxRepository) GetEventsByAggregate(ctx context.Context, aggregateID, aggregateType string) ([]models.OutboxEvent, error) {
	var events []models.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("aggregate_id = ? AND aggregate_type = ?", aggregateID, aggregateType).
		Order("created_at ASC").
		Find(&events).Error

	return events, err
}

// GetEventsByType gets events by type
func (r *outboxRepository) GetEventsByType(ctx context.Context, eventType string, limit, offset int) ([]models.OutboxEvent, error) {
	var events []models.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("event_type = ?", eventType).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&events).Error

	return events, err
}

// CleanupOldEvents removes old events that have been published
func (r *outboxRepository) CleanupOldEvents(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	result := r.db.WithContext(ctx).
		Where("status = ? AND published_at < ?", models.OutboxStatusPublished, cutoffTime).
		Delete(&models.OutboxEvent{})

	return result.Error
}
