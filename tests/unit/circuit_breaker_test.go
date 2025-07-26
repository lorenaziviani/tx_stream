package unit

import (
	"context"
	"testing"
	"time"

	"github.com/lorenaziviani/txstream/internal/infrastructure/kafka"
	"github.com/lorenaziviani/txstream/internal/infrastructure/metrics"
	"github.com/stretchr/testify/assert"
)

func TestCircuitBreakerCreation(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(5, 3, 10*time.Second, 30*time.Second, metrics)

	assert.NotNil(t, cb, "Circuit breaker should not be nil")
	assert.Equal(t, kafka.StateClosed, cb.GetState(), "Initial state should be CLOSED")
	assert.Equal(t, 0, cb.GetFailureCount(), "Initial failure count should be 0")
	assert.Equal(t, 0, cb.GetSuccessCount(), "Initial success count should be 0")
}

func TestCircuitBreakerClosedState(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(3, 2, 5*time.Second, 10*time.Second, metrics)

	err := cb.Execute(context.Background(), func() error {
		return nil
	})

	assert.NoError(t, err, "Successful execution should not return error")
	assert.Equal(t, kafka.StateClosed, cb.GetState(), "State should remain CLOSED")
	assert.Equal(t, 0, cb.GetFailureCount(), "Failure count should remain 0")
	assert.Equal(t, 1, cb.GetSuccessCount(), "Success count should be 1")
}

func TestCircuitBreakerFailureThreshold(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(2, 1, 5*time.Second, 10*time.Second, metrics)

	for i := 0; i < 2; i++ {
		err := cb.Execute(context.Background(), func() error {
			return assert.AnError
		})
		assert.Error(t, err, "Failing execution should return error")
	}

	// Circuit should be open after failure threshold
	assert.Equal(t, kafka.StateOpen, cb.GetState(), "Circuit should be OPEN after failure threshold")
	assert.Equal(t, 2, cb.GetFailureCount(), "Failure count should be 2")
}

func TestCircuitBreakerOpenState(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(2, 1, 5*time.Second, 10*time.Second, metrics)

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return assert.AnError
		})
	}

	assert.Equal(t, kafka.StateOpen, cb.GetState(), "Circuit should be OPEN")

	// In open state, operations should fail immediately
	err := cb.Execute(context.Background(), func() error {
		return nil
	})

	assert.Error(t, err, "Operations should fail immediately in OPEN state")
	assert.Contains(t, err.Error(), "circuit breaker is OPEN", "Error should indicate circuit is open")
}

func TestCircuitBreakerTimeout(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(2, 1, 5*time.Second, 100*time.Millisecond, metrics)

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return assert.AnError
		})
	}

	assert.Equal(t, kafka.StateOpen, cb.GetState(), "Circuit should be OPEN")

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Execute a dummy function to trigger the state transition check
	cb.Execute(context.Background(), func() error {
		return nil
	})

	// Circuit should transition to half-open and then close (since success threshold is 1)
	assert.Equal(t, kafka.StateClosed, cb.GetState(), "Circuit should be CLOSED after successful execution in half-open")
}

func TestCircuitBreakerHalfOpenSuccess(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(2, 2, 5*time.Second, 100*time.Millisecond, metrics)

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return assert.AnError
		})
	}

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Successful execution in half-open state
	err := cb.Execute(context.Background(), func() error {
		return nil
	})

	assert.NoError(t, err, "Successful execution in half-open should not return error")
	assert.Equal(t, kafka.StateHalfOpen, cb.GetState(), "Circuit should remain HALF_OPEN after first success")
	assert.Equal(t, 0, cb.GetFailureCount(), "Failure count should be reset to 0")
	assert.Equal(t, 1, cb.GetSuccessCount(), "Success count should be 1")

	// Second successful execution should close the circuit
	err = cb.Execute(context.Background(), func() error {
		return nil
	})

	assert.NoError(t, err, "Second successful execution should not return error")
	assert.Equal(t, kafka.StateClosed, cb.GetState(), "Circuit should be CLOSED after success threshold")
	assert.Equal(t, 0, cb.GetFailureCount(), "Failure count should remain 0")
	assert.Equal(t, 2, cb.GetSuccessCount(), "Success count should be 2")
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(2, 2, 5*time.Second, 100*time.Millisecond, metrics)

	// Trigger circuit to open
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return assert.AnError
		})
	}

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Failed execution in half-open state
	err := cb.Execute(context.Background(), func() error {
		return assert.AnError
	})

	assert.Error(t, err, "Failed execution in half-open should return error")
	assert.Equal(t, kafka.StateOpen, cb.GetState(), "Circuit should return to OPEN after failure")
	assert.Equal(t, 1, cb.GetFailureCount(), "Failure count should be 1")
	assert.Equal(t, 0, cb.GetSuccessCount(), "Success count should be 0")
}

func TestCircuitBreakerForceMethods(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(5, 3, 10*time.Second, 30*time.Second, metrics)

	// Test force open
	cb.ForceOpen()
	assert.Equal(t, kafka.StateOpen, cb.GetState(), "Circuit should be OPEN after force open")

	// Test force close
	cb.ForceClose()
	assert.Equal(t, kafka.StateClosed, cb.GetState(), "Circuit should be CLOSED after force close")
}

func TestCircuitBreakerStats(t *testing.T) {
	metrics := metrics.NewMetrics()
	cb := kafka.NewCircuitBreaker(3, 2, 5*time.Second, 10*time.Second, metrics)

	stats := cb.GetStats()

	assert.NotNil(t, stats, "Stats should not be nil")
	assert.Contains(t, stats, "state", "Stats should contain state")
	assert.Contains(t, stats, "failure_count", "Stats should contain failure_count")
	assert.Contains(t, stats, "success_count", "Stats should contain success_count")
	assert.Contains(t, stats, "failure_threshold", "Stats should contain failure_threshold")
	assert.Contains(t, stats, "success_threshold", "Stats should contain success_threshold")
	assert.Contains(t, stats, "timeout_duration", "Stats should contain timeout_duration")
	assert.Contains(t, stats, "reset_timeout", "Stats should contain reset_timeout")

	// Test initial values
	assert.Equal(t, "CLOSED", stats["state"], "Initial state should be CLOSED")
	assert.Equal(t, 0, stats["failure_count"], "Initial failure count should be 0")
	assert.Equal(t, 0, stats["success_count"], "Initial success count should be 0")
	assert.Equal(t, 3, stats["failure_threshold"], "Failure threshold should be 3")
	assert.Equal(t, 2, stats["success_threshold"], "Success threshold should be 2")
}
