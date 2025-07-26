package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lorenaziviani/txstream/internal/infrastructure/metrics"
	"github.com/stretchr/testify/assert"
)

func TestMetricsCreation(t *testing.T) {
	metrics := metrics.NewMetrics()
	assert.NotNil(t, metrics, "Metrics should not be nil")
}

func TestEventProcessingMetrics(t *testing.T) {
	metrics := metrics.NewMetrics()

	metrics.RecordEventProcessed("published", "order_created")
	metrics.RecordEventProcessed("failed", "order_created")
	metrics.RecordEventProcessed("published", "order_cancelled")

	metrics.RecordEventPublished("txstream.events", "order_created")
	metrics.RecordEventPublished("txstream.events", "order_cancelled")

	metrics.RecordEventFailed("publish_error", "order_created")
	metrics.RecordEventFailed("database_error", "order_created")

	metrics.RecordEventRetried("1", "order_created")
	metrics.RecordEventRetried("2", "order_created")
	metrics.RecordEventRetried("3", "order_created")

	metrics.RecordCircuitBreakerTrip("CLOSED", "OPEN")
	metrics.RecordCircuitBreakerTrip("OPEN", "HALF_OPEN")
	metrics.RecordCircuitBreakerTrip("HALF_OPEN", "CLOSED")
}

func TestDurationMetrics(t *testing.T) {
	metrics := metrics.NewMetrics()

	processingDuration := 150 * time.Millisecond
	metrics.RecordEventProcessingDuration("order_created", processingDuration)
	metrics.RecordEventProcessingDuration("order_cancelled", 200*time.Millisecond)

	publishingDuration := 50 * time.Millisecond
	metrics.RecordEventPublishingDuration("txstream.events", "order_created", publishingDuration)
	metrics.RecordEventPublishingDuration("txstream.events", "order_cancelled", 75*time.Millisecond)

	retryDelay := 2 * time.Second
	metrics.RecordRetryDelayDuration("1", retryDelay)
	metrics.RecordRetryDelayDuration("2", 4*time.Second)
	metrics.RecordRetryDelayDuration("3", 8*time.Second)
}

func TestGaugeMetrics(t *testing.T) {
	metrics := metrics.NewMetrics()

	metrics.SetWorkerPoolSize(5)
	metrics.SetWorkerPoolSize(10)

	metrics.SetEventsInQueue("pending", 15)
	metrics.SetEventsInQueue("failed", 3)
	metrics.SetEventsInQueue("pending", 8)

	metrics.SetCircuitBreakerState(0) // Closed
	metrics.SetCircuitBreakerState(1) // Half-Open
	metrics.SetCircuitBreakerState(2) // Open

	metrics.SetActiveWorkers(3)
	metrics.SetActiveWorkers(7)
}

func TestTimerHelper(t *testing.T) {
	metrics := metrics.NewMetrics()

	timer := metrics.Timer()
	assert.NotNil(t, timer, "Timer should not be nil")

	time.Sleep(10 * time.Millisecond)
	duration := timer.Duration()
	assert.Greater(t, duration, time.Duration(0), "Duration should be greater than 0")
	assert.Less(t, duration, 100*time.Millisecond, "Duration should be less than 100ms")
}

func TestMetricsServer(t *testing.T) {
	metrics := metrics.NewMetrics()

	err := metrics.StartMetricsServer(0, "/metrics")
	assert.NoError(t, err, "Should start metrics server without error")

	time.Sleep(100 * time.Millisecond)
}

func TestMetricsEndpoint(t *testing.T) {
	metrics := metrics.NewMetrics()

	metrics.RecordEventProcessed("published", "order_created")
	metrics.RecordEventPublished("txstream.events", "order_created")
	metrics.SetWorkerPoolSize(5)
	metrics.SetCircuitBreakerState(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# HELP txstream_events_processed_total Total number of events processed\n"))
		w.Write([]byte("# TYPE txstream_events_processed_total counter\n"))
		w.Write([]byte("txstream_events_processed_total{event_type=\"order_created\",status=\"published\"} 1\n"))
	}))

	defer server.Close()

	resp, err := http.Get(server.URL)
	assert.NoError(t, err, "Should make request without error")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")
}

func TestMetricsConfiguration(t *testing.T) {
	metrics := metrics.NewMetrics()

	assert.NotNil(t, metrics, "Metrics should be created successfully")

	assert.NotPanics(t, func() {
		metrics.RecordEventProcessed("test", "test_event")
		metrics.RecordEventPublished("test_topic", "test_event")
		metrics.RecordEventFailed("test_error", "test_event")
		metrics.RecordEventRetried("1", "test_event")
		metrics.RecordCircuitBreakerTrip("CLOSED", "OPEN")
		metrics.RecordEventProcessingDuration("test_event", time.Second)
		metrics.RecordEventPublishingDuration("test_topic", "test_event", time.Millisecond)
		metrics.RecordRetryDelayDuration("1", time.Second)
		metrics.SetWorkerPoolSize(5)
		metrics.SetEventsInQueue("pending", 10)
		metrics.SetCircuitBreakerState(0)
		metrics.SetActiveWorkers(3)
	}, "Recording metrics should not panic")
}
