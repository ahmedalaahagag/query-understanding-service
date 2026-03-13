package adaptive

import (
	"context"
	"time"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/hybrid"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/native"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Pipeline runs v3 (native OS) as a fast path, then scores the result.
// If the query is complex, it escalates to v2 (LLM hybrid) for semantic
// understanding. If the LLM fails, it falls back to the v3 result.
type Pipeline struct {
	v3        *native.Pipeline
	v2        *hybrid.Pipeline
	scorerCfg ScorerConfig
	metrics   *Metrics
	logger    *logrus.Logger
}

// Metrics holds Prometheus counters for adaptive pipeline observability.
type Metrics struct {
	EscalationsTotal prometheus.Counter
	V3Total          prometheus.Counter
	V2Total          prometheus.Counter
	V2FallbackTotal  prometheus.Counter
	ScoreHistogram   prometheus.Histogram
}

// PipelineConfig holds dependencies for constructing an adaptive pipeline.
type PipelineConfig struct {
	V3        *native.Pipeline
	V2        *hybrid.Pipeline
	ScorerCfg ScorerConfig
	Metrics   *Metrics
	Logger    *logrus.Logger
}

// NewPipeline creates an adaptive pipeline that routes between v3 and v2.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &Pipeline{
		v3:        cfg.V3,
		v2:        cfg.V2,
		scorerCfg: cfg.ScorerCfg,
		metrics:   cfg.Metrics,
		logger:    logger,
	}
}

// Result wraps the response with routing metadata.
type Result struct {
	Response   model.AnalyzeResponse
	Score      ComplexityScore
	Escalated  bool
	V2Fallback bool
}

// Run executes the adaptive pipeline.
func (p *Pipeline) Run(ctx context.Context, req model.AnalyzeRequest) Result {
	start := time.Now()

	// Fast path: run v3 native pipeline.
	v3Resp, v3Err := p.v3.Run(ctx, req)
	if v3Err != nil {
		p.logger.WithError(v3Err).WithField("query", req.Query).Warn("adaptive: v3 failed, escalating to v2")
		return p.escalate(ctx, req, v3Resp, ComplexityScore{Escalate: true}, start)
	}

	// Score v3 output to decide if escalation is needed.
	cs := Score(v3Resp, req.Query, p.scorerCfg)

	if p.metrics != nil {
		p.metrics.ScoreHistogram.Observe(cs.Score)
	}

	if !cs.Escalate {
		if p.metrics != nil {
			p.metrics.V3Total.Inc()
		}
		p.logger.WithFields(logrus.Fields{
			"query":    req.Query,
			"score":    cs.Score,
			"duration": time.Since(start).String(),
		}).Debug("adaptive: v3 sufficient")
		return Result{Response: v3Resp, Score: cs}
	}

	return p.escalate(ctx, req, v3Resp, cs, start)
}

func (p *Pipeline) escalate(ctx context.Context, req model.AnalyzeRequest, v3Resp model.AnalyzeResponse, cs ComplexityScore, start time.Time) Result {
	if p.metrics != nil {
		p.metrics.EscalationsTotal.Inc()
	}

	if p.v2 == nil {
		p.logger.WithField("query", req.Query).Warn("adaptive: escalation needed but v2 not available")
		if p.metrics != nil {
			p.metrics.V3Total.Inc()
		}
		return Result{Response: v3Resp, Score: cs, Escalated: true, V2Fallback: true}
	}

	v2Resp, _ := p.v2.Run(ctx, req, false)

	// If v2 returned no tokens AND no filters, it produced nothing useful.
	// But 0 tokens + filters is a valid filter-only query (e.g. "show me something easy").
	if len(v2Resp.Tokens) == 0 && len(v2Resp.Filters) == 0 {
		p.logger.WithField("query", req.Query).Warn("adaptive: v2 returned empty, falling back to v3")
		if p.metrics != nil {
			p.metrics.V2FallbackTotal.Inc()
			p.metrics.V3Total.Inc()
		}
		return Result{Response: v3Resp, Score: cs, Escalated: true, V2Fallback: true}
	}

	if p.metrics != nil {
		p.metrics.V2Total.Inc()
	}

	p.logger.WithFields(logrus.Fields{
		"query":    req.Query,
		"score":    cs.Score,
		"concepts": len(v2Resp.Concepts),
		"filters":  len(v2Resp.Filters),
		"duration": time.Since(start).String(),
	}).Info("adaptive: escalated to v2")

	return Result{Response: v2Resp, Score: cs, Escalated: true}
}
