package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lorenaziviani/txstream/internal/infrastructure/kafka"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("new_circuit_breaker_initialization", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(5, 3, 10*time.Second, 30*time.Second)

		assert.Equal(t, kafka.StateClosed, cb.GetState())
		assert.Equal(t, 0, cb.GetFailureCount())
		assert.Equal(t, 0, cb.GetSuccessCount())
		assert.Equal(t, 5, cb.GetFailureThreshold())
		assert.Equal(t, 3, cb.GetSuccessThreshold())
		assert.Equal(t, 10*time.Second, cb.GetTimeoutDuration())
		assert.Equal(t, 30*time.Second, cb.GetResetTimeout())
	})

	t.Run("circuit_breaker_closed_state_execution", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(3, 2, 5*time.Second, 10*time.Second)

		err := cb.Execute(context.Background(), func() error {
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, kafka.StateClosed, cb.GetState())
		assert.Equal(t, 0, cb.GetFailureCount())
		assert.Equal(t, 1, cb.GetSuccessCount())
	})

	t.Run("circuit_breaker_failure_threshold_reached", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(2, 1, 5*time.Second, 10*time.Second)

		err := cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})
		assert.Error(t, err)
		assert.Equal(t, kafka.StateClosed, cb.GetState())
		assert.Equal(t, 1, cb.GetFailureCount())

		// Second failure - should open circuit
		err = cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})
		assert.Error(t, err)
		assert.Equal(t, kafka.StateOpen, cb.GetState())
		assert.Equal(t, 2, cb.GetFailureCount())
	})

	t.Run("circuit_breaker_open_state_blocks_execution", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(1, 1, 5*time.Second, 10*time.Second)

		cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})

		assert.Equal(t, kafka.StateOpen, cb.GetState())

		err := cb.Execute(context.Background(), func() error {
			return nil
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is OPEN")
	})

	t.Run("circuit_breaker_reset_timeout_transitions_to_half_open", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(1, 2, 5*time.Second, 100*time.Millisecond)

		cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})

		assert.Equal(t, kafka.StateOpen, cb.GetState())

		time.Sleep(150 * time.Millisecond)

		// Should transition to half-open and allow execution
		err := cb.Execute(context.Background(), func() error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, kafka.StateHalfOpen, cb.GetState())
		// In half-open state, counters should be reset
		assert.Equal(t, 0, cb.GetFailureCount())
		assert.Equal(t, 1, cb.GetSuccessCount())
	})

	t.Run("circuit_breaker_half_open_success_transitions_to_closed", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(1, 2, 5*time.Second, 100*time.Millisecond)

		cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})

		time.Sleep(150 * time.Millisecond)

		// First success in half-open
		err := cb.Execute(context.Background(), func() error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, kafka.StateHalfOpen, cb.GetState())

		// Second success should close circuit
		err = cb.Execute(context.Background(), func() error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, kafka.StateClosed, cb.GetState())
	})

	t.Run("circuit_breaker_half_open_failure_transitions_to_open", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(1, 2, 5*time.Second, 100*time.Millisecond)

		cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})

		time.Sleep(150 * time.Millisecond)

		// Failure in half-open should open circuit again
		err := cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})
		assert.Error(t, err)
		assert.Equal(t, kafka.StateOpen, cb.GetState())
	})

	t.Run("circuit_breaker_success_resets_failure_count", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(3, 1, 5*time.Second, 10*time.Second)

		cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})
		cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})

		assert.Equal(t, 2, cb.GetFailureCount())
		assert.Equal(t, kafka.StateClosed, cb.GetState())

		// Success should reset failure count
		cb.Execute(context.Background(), func() error {
			return nil
		})

		assert.Equal(t, 0, cb.GetFailureCount())
		assert.Equal(t, kafka.StateClosed, cb.GetState())
	})

	t.Run("circuit_breaker_get_stats", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(2, 1, 5*time.Second, 10*time.Second)

		stats := cb.GetStats()
		assert.Equal(t, "CLOSED", stats["state"])
		assert.Equal(t, 0, stats["failure_count"])
		assert.Equal(t, 0, stats["success_count"])
		assert.Equal(t, 2, stats["failure_threshold"])
		assert.Equal(t, 1, stats["success_threshold"])
		assert.Equal(t, 5*time.Second, stats["timeout_duration"])
		assert.Equal(t, 10*time.Second, stats["reset_timeout"])
	})

	t.Run("circuit_breaker_force_open", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(5, 3, 5*time.Second, 10*time.Second)

		assert.Equal(t, kafka.StateClosed, cb.GetState())

		cb.ForceOpen()
		assert.Equal(t, kafka.StateOpen, cb.GetState())
	})

	t.Run("circuit_breaker_force_close", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(1, 1, 5*time.Second, 10*time.Second)

		cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})
		assert.Equal(t, kafka.StateOpen, cb.GetState())

		cb.ForceClose()
		assert.Equal(t, kafka.StateClosed, cb.GetState())
	})

	t.Run("circuit_breaker_state_check_methods", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(1, 1, 5*time.Second, 10*time.Second)

		assert.True(t, cb.IsClosed())
		assert.False(t, cb.IsOpen())
		assert.False(t, cb.IsHalfOpen())

		// Open the circuit
		cb.ForceOpen()
		assert.False(t, cb.IsClosed())
		assert.True(t, cb.IsOpen())
		assert.False(t, cb.IsHalfOpen())

		// Force half-open
		cb.ForceClose()
		cb.ForceOpen()
		cb.Execute(context.Background(), func() error {
			return nil
		})
	})

	t.Run("circuit_breaker_state_change_callback", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(1, 1, 5*time.Second, 10*time.Second)

		var stateChanges []string
		cb.SetStateChangeCallback(func(from, to kafka.CircuitBreakerState) {
			stateChanges = append(stateChanges, from.String()+"->"+to.String())
		})

		cb.ForceOpen()
		cb.ForceClose()

		assert.Len(t, stateChanges, 2)
		assert.Equal(t, "CLOSED->OPEN", stateChanges[0])
		assert.Equal(t, "OPEN->CLOSED", stateChanges[1])
	})

	t.Run("circuit_breaker_concurrent_access", func(t *testing.T) {
		cb := kafka.NewCircuitBreaker(10, 5, 5*time.Second, 10*time.Second)

		// Test concurrent execution
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				cb.Execute(context.Background(), func() error {
					return nil
				})
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		// Should still be in closed state
		assert.Equal(t, kafka.StateClosed, cb.GetState())
		assert.Equal(t, 10, cb.GetSuccessCount())
	})
}
