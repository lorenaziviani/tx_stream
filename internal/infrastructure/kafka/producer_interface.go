package kafka

import (
	"context"

	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/models"
)

type EventProducer interface {
	PublishEvent(ctx context.Context, event *models.OutboxEvent) error
	Close() error
	IsConnected() bool
	GetConfig() *config.KafkaConfig

	GetCircuitBreakerStats() map[string]interface{}
	IsCircuitBreakerOpen() bool
	IsCircuitBreakerHalfOpen() bool
	ForceCircuitBreakerOpen()
	ForceCircuitBreakerClose()
}
