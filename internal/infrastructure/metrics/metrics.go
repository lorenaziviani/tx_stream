package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	eventsProcessedTotal *prometheus.CounterVec
	eventsPublishedTotal *prometheus.CounterVec
	eventsFailedTotal    *prometheus.CounterVec
	eventsRetriedTotal   *prometheus.CounterVec
	circuitBreakerTrips  *prometheus.CounterVec

	eventProcessingDuration *prometheus.HistogramVec
	eventPublishingDuration *prometheus.HistogramVec
	retryDelayDuration      *prometheus.HistogramVec

	workerPoolSize      *prometheus.GaugeVec
	eventsInQueue       *prometheus.GaugeVec
	circuitBreakerState *prometheus.GaugeVec
	activeWorkers       *prometheus.GaugeVec

	registry *prometheus.Registry
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()

	metrics := &Metrics{
		registry: registry,

		eventsProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "txstream_events_processed_total",
				Help: "Total number of events processed by the worker",
			},
			[]string{"status", "event_type"},
		),

		eventsPublishedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "txstream_events_published_total",
				Help: "Total number of events successfully published to Kafka",
			},
			[]string{"topic", "event_type"},
		),

		eventsFailedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "txstream_events_failed_total",
				Help: "Total number of events that failed to be published",
			},
			[]string{"error_type", "event_type"},
		),

		eventsRetriedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "txstream_events_retried_total",
				Help: "Total number of events that were retried",
			},
			[]string{"retry_count", "event_type"},
		),

		circuitBreakerTrips: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "txstream_circuit_breaker_trips_total",
				Help: "Total number of circuit breaker state changes",
			},
			[]string{"from_state", "to_state"},
		),

		// Histograms
		eventProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "txstream_event_processing_duration_seconds",
				Help:    "Time spent processing events",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"event_type"},
		),

		eventPublishingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "txstream_event_publishing_duration_seconds",
				Help:    "Time spent publishing events to Kafka",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"topic", "event_type"},
		),

		retryDelayDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "txstream_retry_delay_duration_seconds",
				Help:    "Duration of retry delays",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"retry_attempt"},
		),

		workerPoolSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "txstream_worker_pool_size",
				Help: "Current size of the worker pool",
			},
			[]string{},
		),

		eventsInQueue: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "txstream_events_in_queue",
				Help: "Number of events currently in the processing queue",
			},
			[]string{"status"},
		),

		circuitBreakerState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "txstream_circuit_breaker_state",
				Help: "Current state of the circuit breaker (0=Closed, 1=Half-Open, 2=Open)",
			},
			[]string{},
		),

		activeWorkers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "txstream_active_workers",
				Help: "Number of currently active workers",
			},
			[]string{},
		),
	}

	registry.MustRegister(
		metrics.eventsProcessedTotal,
		metrics.eventsPublishedTotal,
		metrics.eventsFailedTotal,
		metrics.eventsRetriedTotal,
		metrics.circuitBreakerTrips,
		metrics.eventProcessingDuration,
		metrics.eventPublishingDuration,
		metrics.retryDelayDuration,
		metrics.workerPoolSize,
		metrics.eventsInQueue,
		metrics.circuitBreakerState,
		metrics.activeWorkers,
	)

	return metrics
}

// StartMetricsServer starts the Prometheus metrics server
func (m *Metrics) StartMetricsServer(port int, path string) error {
	http.Handle(path, promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	addr := fmt.Sprintf(":%d", port)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			panic(fmt.Sprintf("Failed to start metrics server: %v", err))
		}
	}()

	return nil
}

// EventProcessing methods
func (m *Metrics) RecordEventProcessed(status, eventType string) {
	m.eventsProcessedTotal.WithLabelValues(status, eventType).Inc()
}

func (m *Metrics) RecordEventPublished(topic, eventType string) {
	m.eventsPublishedTotal.WithLabelValues(topic, eventType).Inc()
}

func (m *Metrics) RecordEventFailed(errorType, eventType string) {
	m.eventsFailedTotal.WithLabelValues(errorType, eventType).Inc()
}

func (m *Metrics) RecordEventRetried(retryCount, eventType string) {
	m.eventsRetriedTotal.WithLabelValues(retryCount, eventType).Inc()
}

func (m *Metrics) RecordCircuitBreakerTrip(fromState, toState string) {
	m.circuitBreakerTrips.WithLabelValues(fromState, toState).Inc()
}

// Duration methods
func (m *Metrics) RecordEventProcessingDuration(eventType string, duration time.Duration) {
	m.eventProcessingDuration.WithLabelValues(eventType).Observe(duration.Seconds())
}

func (m *Metrics) RecordEventPublishingDuration(topic, eventType string, duration time.Duration) {
	m.eventPublishingDuration.WithLabelValues(topic, eventType).Observe(duration.Seconds())
}

func (m *Metrics) RecordRetryDelayDuration(retryAttempt string, duration time.Duration) {
	m.retryDelayDuration.WithLabelValues(retryAttempt).Observe(duration.Seconds())
}

// Gauge methods
func (m *Metrics) SetWorkerPoolSize(size int) {
	m.workerPoolSize.WithLabelValues().Set(float64(size))
}

func (m *Metrics) SetEventsInQueue(status string, count int) {
	m.eventsInQueue.WithLabelValues(status).Set(float64(count))
}

func (m *Metrics) SetCircuitBreakerState(state int) {
	m.circuitBreakerState.WithLabelValues().Set(float64(state))
}

func (m *Metrics) SetActiveWorkers(count int) {
	m.activeWorkers.WithLabelValues().Set(float64(count))
}

// Timer helper for measuring durations
func (m *Metrics) Timer() *Timer {
	return &Timer{
		start: time.Now(),
	}
}

// Timer helps measure durations
type Timer struct {
	start time.Time
}

func (t *Timer) Duration() time.Duration {
	return time.Since(t.start)
}

// ContextWithTimer adds a timer to the context
func ContextWithTimer(ctx context.Context, metrics *Metrics) (context.Context, *Timer) {
	timer := metrics.Timer()
	return context.WithValue(ctx, "timer", timer), timer
}

// TimerFromContext extracts timer from context
func TimerFromContext(ctx context.Context) *Timer {
	if timer, ok := ctx.Value("timer").(*Timer); ok {
		return timer
	}
	return nil
}
