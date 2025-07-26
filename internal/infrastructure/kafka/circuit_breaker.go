package kafka

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lorenaziviani/txstream/internal/infrastructure/metrics"
)

type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu sync.RWMutex

	failureThreshold int
	successThreshold int
	timeoutDuration  time.Duration
	resetTimeout     time.Duration

	state CircuitBreakerState

	failureCount int
	successCount int

	lastFailureTime time.Time
	lastStateChange time.Time

	onStateChange func(from, to CircuitBreakerState)
	metrics       *metrics.Metrics
}

// NewCircuitBreaker creates a new CircuitBreaker instance
func NewCircuitBreaker(failureThreshold, successThreshold int, timeoutDuration, resetTimeout time.Duration, metrics *metrics.Metrics) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeoutDuration:  timeoutDuration,
		resetTimeout:     resetTimeout,
		failureCount:     0,
		successCount:     0,
		lastFailureTime:  time.Time{},
		lastStateChange:  time.Now(),
		mu:               sync.RWMutex{},
		metrics:          metrics,
	}

	if metrics != nil {
		metrics.SetCircuitBreakerState(0) // 0 = Closed
	}

	cb.onStateChange = func(from, to CircuitBreakerState) {
		log.Printf("Circuit Breaker state changed from %s to %s", from.String(), to.String())
	}

	return cb
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker is %s", cb.getState())
	}

	err := fn()

	cb.recordResult(err)

	return err
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailureTime) >= cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.transitionTo(StateHalfOpen)
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of an execution
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failure
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.successCount = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		cb.transitionTo(StateOpen)
	}
}

// recordSuccess records a success
func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++
	cb.failureCount = 0

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0
	case StateHalfOpen:
		if cb.successCount >= cb.successThreshold {
			cb.transitionTo(StateClosed)
		}
	}
}

// transitionTo transitions the circuit breaker to a new state
func (cb *CircuitBreaker) transitionTo(newState CircuitBreakerState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	// Record state change in metrics
	if cb.metrics != nil {
		cb.metrics.RecordCircuitBreakerTrip(oldState.String(), newState.String())

		// Update circuit breaker state gauge
		var stateValue int
		switch newState {
		case StateClosed:
			stateValue = 0
		case StateHalfOpen:
			stateValue = 1
		case StateOpen:
			stateValue = 2
		}
		cb.metrics.SetCircuitBreakerState(stateValue)
	}

	// Reset counters when entering half-open state
	if newState == StateHalfOpen {
		cb.failureCount = 0
		cb.successCount = 0
	}

	if cb.onStateChange != nil {
		cb.onStateChange(oldState, newState)
	}
}

// GetState returns the current state (thread-safe)
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// getState returns the current state (thread-safe) - internal use
func (cb *CircuitBreaker) getState() CircuitBreakerState {
	return cb.GetState()
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":             cb.state.String(),
		"failure_count":     cb.failureCount,
		"success_count":     cb.successCount,
		"last_failure_time": cb.lastFailureTime,
		"last_state_change": cb.lastStateChange,
		"failure_threshold": cb.failureThreshold,
		"success_threshold": cb.successThreshold,
		"timeout_duration":  cb.timeoutDuration,
		"reset_timeout":     cb.resetTimeout,
	}
}

// SetStateChangeCallback sets a custom callback for state changes
func (cb *CircuitBreaker) SetStateChangeCallback(callback func(from, to CircuitBreakerState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = callback
}

// ForceOpen forces the circuit breaker to open state
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(StateOpen)
}

// ForceClose forces the circuit breaker to closed state
func (cb *CircuitBreaker) ForceClose() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(StateClosed)
}

// IsOpen returns true if the circuit breaker is open
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.getState() == StateOpen
}

// IsClosed returns true if the circuit breaker is closed
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.getState() == StateClosed
}

// IsHalfOpen returns true if the circuit breaker is half-open
func (cb *CircuitBreaker) IsHalfOpen() bool {
	return cb.getState() == StateHalfOpen
}

// GetFailureCount returns the current failure count (for testing)
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// GetSuccessCount returns the current success count (for testing)
func (cb *CircuitBreaker) GetSuccessCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.successCount
}

// GetFailureThreshold returns the failure threshold (for testing)
func (cb *CircuitBreaker) GetFailureThreshold() int {
	return cb.failureThreshold
}

// GetSuccessThreshold returns the success threshold (for testing)
func (cb *CircuitBreaker) GetSuccessThreshold() int {
	return cb.successThreshold
}

// GetTimeoutDuration returns the timeout duration (for testing)
func (cb *CircuitBreaker) GetTimeoutDuration() time.Duration {
	return cb.timeoutDuration
}

// GetResetTimeout returns the reset timeout (for testing)
func (cb *CircuitBreaker) GetResetTimeout() time.Duration {
	return cb.resetTimeout
}
