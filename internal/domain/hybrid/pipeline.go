package hybrid

import (
	"context"
	"strings"
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
	comprehension   *pipeline.ComprehensionEngine
	stopwords       map[string]map[string]bool
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
	Comprehension   *pipeline.ComprehensionEngine
	Stopwords       map[string]map[string]bool
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
		comprehension:   cfg.Comprehension,
		stopwords:       cfg.Stopwords,
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

		// Run comprehension on fallback so filter/sort tokens (e.g. "cheap",
		// "under 20") are stripped before the orchestrator builds a text query.
		if p.comprehension != nil {
			p.comprehension.Process(ctx, state)
		}
		if sw := p.stopwordsForLocale(req.Locale); len(sw) > 0 {
			state.Tokens = filterStopwords(state.Tokens, sw)
		}

		if p.failOpen {
			resp := BuildFallbackResponse(state, err.Error())
			if debug {
				debugInfo.FallbackUsed = true
				debugInfo.Warnings = resp.Warnings
				debugInfo.Duration = time.Since(start).String()
			}
			return resp, debugInfo
		}
		return BuildFallbackResponse(state, err.Error()), debugInfo
	}

	if p.metrics != nil {
		p.metrics.LLMRequestsTotal.WithLabelValues("ok").Inc()
	}

	if debug {
		debugInfo.RawLLMOutput = llmResult
	}

	// Step 3: Validate LLM output
	intent := p.validator.Validate(llmResult, state.NormalizedQuery)

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
			state.NormalizedQuery = normalizedQuery
			pipeline.Tokenizer{}.Process(ctx, state)
		}
	}

	// Strip tokens consumed by filters/sort (e.g. "under 20", "cheap") so
	// only search-relevant terms remain.
	if p.comprehension != nil {
		p.comprehension.Process(ctx, state)
		normalizedQuery = state.NormalizedQuery
	}

	// Remove stopwords so the orchestrator doesn't try to match filler like
	// "something", "for", "with", "and" that would cause 0-hit queries.
	if sw := p.stopwordsForLocale(req.Locale); len(sw) > 0 {
		state.Tokens = filterStopwords(state.Tokens, sw)
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

// stopwordsForLocale returns the stopword set for the given locale.
func (p *Pipeline) stopwordsForLocale(locale string) map[string]bool {
	key := strings.ToLower(strings.ReplaceAll(locale, "-", "_"))
	return p.stopwords[key]
}

// filterStopwords removes tokens whose normalized value is a stopword.
func filterStopwords(tokens []model.Token, stopwords map[string]bool) []model.Token {
	var filtered []model.Token
	for _, t := range tokens {
		if stopwords[strings.ToLower(t.Normalized)] {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}
