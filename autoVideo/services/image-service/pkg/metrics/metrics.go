package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// GenerationDuration tracks image generation latency per model and status.
	GenerationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "image_service",
			Name:      "generation_duration_seconds",
			Help:      "Image generation duration in seconds by model and outcome.",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"model", "status"},
	)

	// GenerationTotal counts completed generation attempts.
	GenerationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "image_service",
			Name:      "generation_total",
			Help:      "Total image generation attempts by model and status.",
		},
		[]string{"model", "status"},
	)

	// QueueDepth tracks how many tasks are waiting in each per-generator semaphore.
	QueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "image_service",
			Name:      "queue_depth",
			Help:      "Current number of in-flight tasks per generator.",
		},
		[]string{"generator"},
	)

	// RateLimitHits counts 429 responses per model.
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "image_service",
			Name:      "rate_limit_hits_total",
			Help:      "Number of 429 rate-limit responses per model.",
		},
		[]string{"model"},
	)

	// KeyPoolHealth tracks available (non-cooldown) keys per generator.
	KeyPoolHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "image_service",
			Name:      "key_pool_available_keys",
			Help:      "Number of API keys not on cooldown per generator.",
		},
		[]string{"generator"},
	)
)