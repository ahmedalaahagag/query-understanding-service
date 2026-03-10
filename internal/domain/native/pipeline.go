package native

import (
	"context"
	"time"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/pipeline"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
)

// Pipeline orchestrates the native OS-driven analysis flow.
// Unlike v1 which handles synonyms/compounds/spell in Go,
// v3 delegates these to OpenSearch fuzzy matching.
type Pipeline struct {
	steps  []pipeline.Step
	logger *logrus.Logger
}

// PipelineConfig holds dependencies for constructing a native pipeline.
type PipelineConfig struct {
	FuzzySearcher   opensearch.FuzzySearcher
	ConceptSearcher opensearch.ConceptSearcher
	Comprehension   config.ComprehensionConfig
	Concept         config.ConceptConfig
	Logger          *logrus.Logger
}

// NewPipeline creates a native pipeline with OS-driven steps.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	return &Pipeline{
		steps: []pipeline.Step{
			pipeline.Normalizer{},
			pipeline.Tokenizer{},
			NewNativeSpellCorrector(cfg.FuzzySearcher, logger),
			NewNativeConceptRecognizer(cfg.FuzzySearcher, cfg.Concept, logger),
			pipeline.AmbiguityResolver{},
			pipeline.NewComprehensionEngine(cfg.Comprehension),
		},
		logger: logger,
	}
}

// Run executes all native pipeline steps and returns the response.
func (p *Pipeline) Run(ctx context.Context, req model.AnalyzeRequest) (model.AnalyzeResponse, error) {
	start := time.Now()

	state := &model.QueryState{
		OriginalQuery:   req.Query,
		NormalizedQuery: req.Query,
		Locale:          req.Locale,
		Market:          req.Market,
	}

	for _, step := range p.steps {
		if err := step.Process(ctx, state); err != nil {
			p.logger.WithError(err).WithField("step", step.Name()).Error("native pipeline step failed")
			return state.ToResponse(), err
		}
	}

	p.logger.WithFields(logrus.Fields{
		"query":      req.Query,
		"normalized": state.NormalizedQuery,
		"tokens":     len(state.Tokens),
		"concepts":   len(state.Concepts),
		"filters":    len(state.Filters),
		"has_sort":   state.Sort != nil,
		"duration":   time.Since(start).String(),
	}).Info("native pipeline completed")

	return state.ToResponse(), nil
}
