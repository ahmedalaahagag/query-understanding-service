package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HybridMetrics holds Prometheus metrics for the LLM hybrid pipeline.
type HybridMetrics struct {
	LLMRequestsTotal      *prometheus.CounterVec
	LLMLatency            prometheus.Histogram
	LLMFailuresTotal      prometheus.Counter
	ValidationRejectsTotal *prometheus.CounterVec
	FallbackTotal         prometheus.Counter
	HybridPipelineDuration prometheus.Histogram
}

// NewHybridMetrics creates and registers LLM hybrid pipeline metrics.
func NewHybridMetrics() *HybridMetrics {
	return &HybridMetrics{
		LLMRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "qus_llm_requests_total",
			Help: "Total number of LLM requests.",
		}, []string{"status"}),

		LLMLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "qus_llm_latency_seconds",
			Help:    "LLM call latency in seconds.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10},
		}),

		LLMFailuresTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "qus_llm_failures_total",
			Help: "Total number of LLM call failures.",
		}),

		ValidationRejectsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "qus_validation_rejects_total",
			Help: "Total number of LLM output fields rejected by validation.",
		}, []string{"type"}),

		FallbackTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "qus_fallback_total",
			Help: "Total number of requests that fell back to deterministic mode.",
		}),

		HybridPipelineDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "qus_hybrid_pipeline_duration_seconds",
			Help:    "Total hybrid pipeline execution duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
	}
}
