package unit

import (
	"testing"
	"time"

	"github.com/lorenaziviani/txstream/internal/infrastructure/config"
	"github.com/lorenaziviani/txstream/internal/infrastructure/kafka"
	"github.com/stretchr/testify/assert"
)

func TestExponentialRetryCalculation(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.KafkaConfig
		attempt     int
		expectedMin time.Duration
		expectedMax time.Duration
		description string
	}{
		{
			name: "exponential_retry_disabled_uses_fixed_delay",
			config: &config.KafkaConfig{
				ExponentialRetryEnabled: false,
				RetryDelay:              2 * time.Second,
			},
			attempt:     1,
			expectedMin: 2 * time.Second,
			expectedMax: 2 * time.Second,
			description: "When exponential retry is disabled, should use fixed retry delay",
		},
		{
			name: "exponential_retry_first_attempt",
			config: &config.KafkaConfig{
				ExponentialRetryEnabled: true,
				BaseDelay:               1 * time.Second,
				Multiplier:              2.0,
				MaxDelay:                30 * time.Second,
			},
			attempt:     0,
			expectedMin: 500 * time.Millisecond,  // 1s * 0.5 (min jitter)
			expectedMax: 1500 * time.Millisecond, // 1s * 1.5 (max jitter)
			description: "First retry should use base delay with jitter",
		},
		{
			name: "exponential_retry_second_attempt",
			config: &config.KafkaConfig{
				ExponentialRetryEnabled: true,
				BaseDelay:               1 * time.Second,
				Multiplier:              2.0,
				MaxDelay:                30 * time.Second,
			},
			attempt:     1,
			expectedMin: 1 * time.Second, // 1s * 2^1 * 0.5 (min jitter)
			expectedMax: 6 * time.Second, // 1s * 2^1 * 1.5 (max jitter)
			description: "Second retry should use base * multiplier with jitter",
		},
		{
			name: "exponential_retry_respects_max_delay",
			config: &config.KafkaConfig{
				ExponentialRetryEnabled: true,
				BaseDelay:               10 * time.Second,
				Multiplier:              2.0,
				MaxDelay:                15 * time.Second,
			},
			attempt:     2,
			expectedMin: 1 * time.Second,  // 10s * 2^2 * 0.5 (min jitter) = 40s * 0.5 = 20s, but capped at 15s
			expectedMax: 15 * time.Second, // Max delay (capped after jitter)
			description: "Should cap delay at max delay even with jitter",
		},
		{
			name: "exponential_retry_custom_multiplier",
			config: &config.KafkaConfig{
				ExponentialRetryEnabled: true,
				BaseDelay:               1 * time.Second,
				Multiplier:              1.5,
				MaxDelay:                30 * time.Second,
			},
			attempt:     2,
			expectedMin: 1 * time.Second, // 1s * 1.5^2 * 0.5 (min jitter)
			expectedMax: 7 * time.Second, // 1s * 1.5^2 * 1.5 (max jitter)
			description: "Should use custom multiplier for exponential calculation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			producer := kafka.NewProducerForTesting(tt.config)

			// Test multiple times to account for jitter randomness
			for i := 0; i < 10; i++ {
				delay := producer.CalculateRetryDelay(tt.attempt)

				assert.GreaterOrEqual(t, delay, tt.expectedMin,
					"Delay should be >= expected minimum: %s", tt.description)
				assert.LessOrEqual(t, delay, tt.expectedMax,
					"Delay should be <= expected maximum: %s", tt.description)
			}
		})
	}
}

func TestExponentialRetryWithJitter(t *testing.T) {
	config := &config.KafkaConfig{
		ExponentialRetryEnabled: true,
		BaseDelay:               1 * time.Second,
		Multiplier:              2.0,
		MaxDelay:                30 * time.Second,
	}

	producer := kafka.NewProducerForTesting(config)

	// Test that jitter produces different delays
	delays := make(map[time.Duration]bool)
	attempt := 1

	for i := 0; i < 20; i++ {
		delay := producer.CalculateRetryDelay(attempt)
		delays[delay] = true
	}

	assert.Greater(t, len(delays), 1, "Jitter should produce different delay values")
}

func TestExponentialRetryConfiguration(t *testing.T) {
	config := &config.KafkaConfig{
		ExponentialRetryEnabled: true,
		MaxRetries:              3,
		BaseDelay:               2 * time.Second,
		MaxDelay:                20 * time.Second,
		Multiplier:              2.5,
	}

	producer := kafka.NewProducerForTesting(config)

	assert.True(t, producer.IsExponentialRetryEnabled(), "Should report exponential retry as enabled")

	retryConfig := producer.GetRetryConfig()
	assert.Equal(t, true, retryConfig["exponential_retry_enabled"])
	assert.Equal(t, 3, retryConfig["max_retries"])
	assert.Equal(t, "2s", retryConfig["base_delay"])
	assert.Equal(t, "20s", retryConfig["max_delay"])
	assert.Equal(t, 2.5, retryConfig["multiplier"])
}

func TestExponentialRetryDisabledConfiguration(t *testing.T) {
	config := &config.KafkaConfig{
		ExponentialRetryEnabled: false,
		MaxRetries:              2,
		RetryDelay:              5 * time.Second,
	}

	producer := kafka.NewProducerForTesting(config)

	assert.False(t, producer.IsExponentialRetryEnabled(), "Should report exponential retry as disabled")

	retryConfig := producer.GetRetryConfig()
	assert.Equal(t, false, retryConfig["exponential_retry_enabled"])
	assert.Equal(t, 2, retryConfig["max_retries"])
	assert.Equal(t, "5s", retryConfig["retry_delay"])
}
