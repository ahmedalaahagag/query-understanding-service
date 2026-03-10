package hybrid

import (
	"context"
	"time"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/pipeline"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/observability"
	"github.com/sirupsen/logrus"
)

// LLMParser is the interface for calling the LLM semantic parser.
type LLMParser interface {
	Parse(ctx context.Context, systemPrompt, userMessage string) (*LLMParseResult, error)
}

// Pipeline orchestrates the hybrid (deterministic + LLM) analysis flow.
type Pipeline struct {
	parser          LLMParser
	promptBuilder   *PromptBuilder
	validator       *Validator
	conceptResolver *ConceptResolver
	metrics         *observability.HybridMetrics
	logger          *logrus.Logger
	failOpen        bool
}

// PipelineConfig holds dependencies for constructing a hybrid pipeline.
type PipelineConfig struct {
	Parser          LLMParser
	PromptBuilder   *PromptBuilder
	Validator       *Validator
	ConceptResolver *ConceptResolver
	Metrics         *observability.HybridMetrics
	Logger          *logrus.Logger
	FailOpen        bool
}

// NewPipeline creates a new hybrid pipeline.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	return &Pipeline{
		parser:          cfg.Parser,
		promptBuilder:   cfg.PromptBuilder,
		validator:       cfg.Validator,
		conceptResolver: cfg.ConceptResolver,
		metrics:         cfg.Metrics,
		logger:          cfg.Logger,
		failOpen:        cfg.FailOpen,
	}
}

// DebugInfo holds intermediate pipeline state for the debug endpoint.
type DebugInfo struct {
	PreprocessedTokens []model.Token      `json:"preprocessedTokens"`
	RawLLMOutput       *LLMParseResult    `json:"rawLlmOutput,omitempty"`
	ValidatedIntent    *ValidatedIntent   `json:"validatedIntent,omitempty"`
	FallbackUsed       bool               `json:"fallbackUsed"`
	Warnings           []string           `json:"warnings,omitempty"`
	Duration           string             `json:"duration"`
}

// Run executes the hybrid pipeline and returns an AnalyzeResponse.
func (p *Pipeline) Run(ctx context.Context, req model.AnalyzeRequest, debug bool) (model.AnalyzeResponse, *DebugInfo) {
	start := time.Now()
	var debugInfo *DebugInfo
	if debug {
		debugInfo = &DebugInfo{}
	}

	// Step 1: Deterministic preprocessing (reuse existing normalizer + tokenizer)
	state := &model.QueryState{
		OriginalQuery:   req.Query,
		NormalizedQuery: req.Query,
	}

	pipeline.Normalizer{}.Process(ctx, state)
	pipeline.Tokenizer{}.Process(ctx, state)

	if debug {
		debugInfo.PreprocessedTokens = state.Tokens
	}

	// Step 2: LLM semantic parse
	systemPrompt := p.promptBuilder.SystemPrompt()
	userMessage := p.promptBuilder.UserMessage(state.NormalizedQuery, req.Locale, req.Market)

	llmStart := time.Now()
	llmResult, err := p.parser.Parse(ctx, systemPrompt, userMessage)
	llmDuration := time.Since(llmStart)

	if p.metrics != nil {
		p.metrics.LLMLatency.Observe(llmDuration.Seconds())
	}

	if err != nil {
		p.logger.WithError(err).WithField("query", req.Query).Warn("LLM parse failed")
		if p.metrics != nil {
			p.metrics.LLMFailuresTotal.Inc()
			p.metrics.LLMRequestsTotal.WithLabelValues("error").Inc()
			p.metrics.FallbackTotal.Inc()
		}

		if p.failOpen {
			resp := BuildFallbackResponse(state.OriginalQuery, state.NormalizedQuery, state.Tokens, err.Error())
			if debug {
				debugInfo.FallbackUsed = true
				debugInfo.Warnings = resp.Warnings
				debugInfo.Duration = time.Since(start).String()
			}
			return resp, debugInfo
		}
		return BuildFallbackResponse(state.OriginalQuery, state.NormalizedQuery, state.Tokens, err.Error()), debugInfo
	}

	if p.metrics != nil {
		p.metrics.LLMRequestsTotal.WithLabelValues("ok").Inc()
	}

	if debug {
		debugInfo.RawLLMOutput = llmResult
	}

	// Step 3: Validate LLM output
	intent := p.validator.Validate(llmResult)

	if debug {
		debugInfo.ValidatedIntent = &intent
	}

	// Record validation rejections
	if p.metrics != nil {
		filterRejections := len(llmResult.Filters) - len(intent.Filters)
		if filterRejections > 0 {
			p.metrics.ValidationRejectsTotal.WithLabelValues("filter").Add(float64(filterRejections))
		}
		conceptRejections := len(llmResult.CandidateConcepts) - len(intent.CandidateConcepts)
		if conceptRejections > 0 {
			p.metrics.ValidationRejectsTotal.WithLabelValues("concept").Add(float64(conceptRejections))
		}
		if llmResult.Sort != nil && intent.Sort == nil {
			p.metrics.ValidationRejectsTotal.WithLabelValues("sort").Inc()
		}
	}

	// Step 4: Resolve concepts via OpenSearch
	concepts, conceptWarnings := p.conceptResolver.Resolve(ctx, intent.CandidateConcepts, state.Tokens, req.Locale, req.Market)

	// Step 5: Assemble final response
	var allWarnings []string
	allWarnings = append(allWarnings, intent.Warnings...)
	allWarnings = append(allWarnings, conceptWarnings...)

	// Use LLM normalized query if available, otherwise keep preprocessed.
	// If the LLM corrected the query (e.g. typo fix), re-tokenize so tokens
	// reflect the corrected form — otherwise the orchestrator searches with
	// the original misspelled tokens.
	normalizedQuery := state.NormalizedQuery
	if intent.NormalizedQuery != "" {
		normalizedQuery = intent.NormalizedQuery
		if normalizedQuery != state.NormalizedQuery {
			reTokenized := &model.QueryState{
				OriginalQuery:   state.OriginalQuery,
				NormalizedQuery: normalizedQuery,
			}
			pipeline.Tokenizer{}.Process(ctx, reTokenized)
			state.Tokens = reTokenized.Tokens
		}
	}

	rewrites := intent.Rewrites
	if rewrites == nil {
		rewrites = []string{}
	}
	if normalizedQuery != state.OriginalQuery {
		found := false
		for _, r := range rewrites {
			if r == normalizedQuery {
				found = true
				break
			}
		}
		if !found {
			rewrites = append([]string{normalizedQuery}, rewrites...)
		}
	}

	if concepts == nil {
		concepts = []model.ConceptMatch{}
	}

	filters := make([]model.Filter, 0, len(intent.Filters))
	for _, f := range intent.Filters {
		filters = append(filters, model.Filter{
			Field:    f.Field,
			Operator: f.Operator,
			Value:    f.Value,
		})
	}

	var sort *model.SortSpec
	if intent.Sort != nil {
		sort = &model.SortSpec{
			Field:     intent.Sort.Field,
			Direction: intent.Sort.Direction,
		}
	}

	tokens := model.MergeConceptTokens(state.Tokens, concepts)
	if tokens == nil {
		tokens = []model.Token{}
	}

	resp := model.AnalyzeResponse{
		OriginalQuery:   state.OriginalQuery,
		NormalizedQuery: normalizedQuery,
		Tokens:          tokens,
		Rewrites:        rewrites,
		Concepts:        concepts,
		Filters:         filters,
		Sort:            sort,
		Warnings:        allWarnings,
	}

	if p.metrics != nil {
		p.metrics.HybridPipelineDuration.Observe(time.Since(start).Seconds())
	}

	if debug {
		debugInfo.Duration = time.Since(start).String()
	}

	p.logger.WithFields(logrus.Fields{
		"query":      req.Query,
		"normalized": normalizedQuery,
		"concepts":   len(concepts),
		"filters":    len(filters),
		"has_sort":   sort != nil,
		"warnings":   len(allWarnings),
		"duration":   time.Since(start).String(),
	}).Info("hybrid pipeline completed")

	return resp, debugInfo
}
