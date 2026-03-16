package adaptive

import (
	"context"
	"strings"
	"time"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/hybrid"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/native"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Pipeline routes queries between v3 (native OS) and v2 (LLM hybrid).
// Short queries (< DirectLLMTokenThreshold non-stopword tokens) go to v3.
// Longer queries go straight to v2 LLM, with v3 as fallback if v2 fails.
type Pipeline struct {
	v3                      *native.Pipeline
	v2                      *hybrid.Pipeline
	directLLMTokenThreshold int
	stopwords               map[string]map[string]bool
	metrics                 *Metrics
	logger                  *logrus.Logger
}

// Metrics holds Prometheus counters for adaptive pipeline observability.
type Metrics struct {
	V3Total         prometheus.Counter
	V2Total         prometheus.Counter
	V2FallbackTotal prometheus.Counter
}

// PipelineConfig holds dependencies for constructing an adaptive pipeline.
type PipelineConfig struct {
	V3                      *native.Pipeline
	V2                      *hybrid.Pipeline
	DirectLLMTokenThreshold int
	Stopwords               map[string]map[string]bool
	Metrics                 *Metrics
	Logger                  *logrus.Logger
}

// NewPipeline creates an adaptive pipeline that routes between v3 and v2.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &Pipeline{
		v3:                      cfg.V3,
		v2:                      cfg.V2,
		directLLMTokenThreshold: cfg.DirectLLMTokenThreshold,
		stopwords:               cfg.Stopwords,
		metrics:                 cfg.Metrics,
		logger:                  logger,
	}
}

// Result wraps the response with routing metadata.
type Result struct {
	Response   model.AnalyzeResponse
	UsedV2     bool
	V2Fallback bool
}

// Run executes the adaptive pipeline.
func (p *Pipeline) Run(ctx context.Context, req model.AnalyzeRequest) Result {
	start := time.Now()

	// Route: if v2 is available and query has enough non-stopword tokens, use LLM.
	if p.directLLMTokenThreshold > 0 && p.v2 != nil {
		if p.countNonStopwordTokens(req.Query, req.Locale) >= p.directLLMTokenThreshold {
			return p.runV2WithFallback(ctx, req, start)
		}
	}

	// Short query: use v3 native pipeline.
	return p.runV3(ctx, req, start)
}

// runV3 runs the native OS pipeline.
func (p *Pipeline) runV3(ctx context.Context, req model.AnalyzeRequest, start time.Time) Result {
	v3Resp, err := p.v3.Run(ctx, req)
	if err != nil {
		p.logger.WithError(err).WithField("query", req.Query).Warn("adaptive: v3 failed")
	}
	if p.metrics != nil {
		p.metrics.V3Total.Inc()
	}
	p.logger.WithFields(logrus.Fields{
		"query":    req.Query,
		"route":    "v3",
		"duration": time.Since(start).String(),
	}).Debug("adaptive: routed to v3")
	return Result{Response: v3Resp}
}

// runV2WithFallback runs v2 LLM, falling back to v3 if v2 produces nothing useful.
func (p *Pipeline) runV2WithFallback(ctx context.Context, req model.AnalyzeRequest, start time.Time) Result {
	v2Resp, _ := p.v2.Run(ctx, req, false)

	// If v2 returned no tokens AND no filters, fall back to v3.
	if len(v2Resp.Tokens) == 0 && len(v2Resp.Filters) == 0 {
		p.logger.WithField("query", req.Query).Warn("adaptive: v2 returned empty, falling back to v3")
		if p.metrics != nil {
			p.metrics.V2FallbackTotal.Inc()
		}
		v3Resp, _ := p.v3.Run(ctx, req)
		if p.metrics != nil {
			p.metrics.V3Total.Inc()
		}
		return Result{Response: v3Resp, UsedV2: true, V2Fallback: true}
	}

	if p.metrics != nil {
		p.metrics.V2Total.Inc()
	}
	p.logger.WithFields(logrus.Fields{
		"query":    req.Query,
		"route":    "v2",
		"concepts": len(v2Resp.Concepts),
		"filters":  len(v2Resp.Filters),
		"duration": time.Since(start).String(),
	}).Info("adaptive: routed to v2")

	return Result{Response: v2Resp, UsedV2: true}
}

// countNonStopwordTokens does a quick token count: lowercase + split + remove stopwords.
func (p *Pipeline) countNonStopwordTokens(query, locale string) int {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(p.stopwords) == 0 {
		return len(words)
	}
	normLocale := strings.ToLower(strings.ReplaceAll(locale, "-", "_"))
	sw := p.stopwords[normLocale]
	if len(sw) == 0 {
		return len(words)
	}
	count := 0
	for _, w := range words {
		if !sw[w] {
			count++
		}
	}
	return count
}
