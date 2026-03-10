package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the QUS service.
type Metrics struct {
	PipelineDuration  *prometheus.HistogramVec
	StepDuration      *prometheus.HistogramVec
	RequestsTotal     *prometheus.CounterVec
	ConceptsFound     prometheus.Histogram
	SpellCorrections  prometheus.Counter
	SynonymExpansions prometheus.Counter
}

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		PipelineDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "qus_pipeline_duration_seconds",
			Help:    "Total pipeline execution duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{}),

		StepDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "qus_pipeline_step_duration_seconds",
			Help:    "Per-step execution duration in seconds.",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		}, []string{"step"}),

		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "qus_requests_total",
			Help: "Total number of analyze requests.",
		}, []string{"status"}),

		ConceptsFound: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "qus_concepts_found",
			Help:    "Number of concepts found per request.",
			Buckets: []float64{0, 1, 2, 3, 5, 10, 20},
		}),

		SpellCorrections: promauto.NewCounter(prometheus.CounterOpts{
			Name: "qus_spell_corrections_total",
			Help: "Total number of spell corrections applied.",
		}),

		SynonymExpansions: promauto.NewCounter(prometheus.CounterOpts{
			Name: "qus_synonym_expansions_total",
			Help: "Total number of synonym expansions applied.",
		}),
	}
}
