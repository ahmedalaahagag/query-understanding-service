// Package analyzer provides the public API for running query understanding analysis.
// Import this package to use QUS as a library instead of calling it over HTTP.
package analyzer

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/adaptive"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/hybrid"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/native"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/pipeline"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/bedrock"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/observability"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
)

// supportedLocales lists all locales to load stopwords for at startup.
var supportedLocales = []string{
	"en_gb", "en_us", "en_ca", "en_au", "en_ie", "en_nz",
	"en_be", "en_de", "en_dk", "en_nl", "en_se",
	"de_de", "de_at", "de_ch",
	"fr_fr", "fr_ca", "fr_be", "fr_lu",
	"nl_nl", "nl_be",
	"it_it", "es_es", "sv_se", "da_dk", "nb_no", "ja_jp",
}

// Analyzer runs query understanding analysis in-process.
type Analyzer struct {
	v1     *pipeline.Pipeline
	v2     *hybrid.Pipeline
	v3     *native.Pipeline
	v4     *adaptive.Pipeline
	logger *logrus.Logger
}

// Config holds all settings needed to construct an Analyzer.
type Config struct {
	// ConfigDir is the path to the configs directory (allowed_filters.yaml, etc.).
	ConfigDir string

	// OpenSearch connection settings.
	OpenSearch config.OpenSearchConfig

	// LLM settings (optional). Leave Enabled=false to use v1 only.
	LLM config.LLMConfig

	// Logger (optional, defaults to logrus standard logger).
	Logger *logrus.Logger
}

// New constructs an Analyzer with both v1 and (optionally) v2 pipelines.
func New(ctx context.Context, cfg Config) (*Analyzer, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	osClient := opensearch.NewClient(opensearch.ClientConfig{
		URL:                   cfg.OpenSearch.URL,
		Username:              cfg.OpenSearch.Username,
		Password:              cfg.OpenSearch.Password,
		ConceptIndexPrefix:    cfg.OpenSearch.ConceptIndexPrefix,
		LinguisticIndexPrefix: cfg.OpenSearch.LinguisticIndexPrefix,
		Timeout:               cfg.OpenSearch.Timeout,
	})

	pipelineCfg, err := config.LoadPipelineConfig(filepath.Join(cfg.ConfigDir, "qus.yaml"))
	if err != nil {
		logger.WithError(err).Warn("could not load pipeline config, using defaults")
		pipelineCfg = config.PipelineConfig{
			Spell: config.SpellConfig{
				Enabled:             true,
				MinTokenLength:      4,
				ConfidenceThreshold: 0.85,
			},
		}
	}

	comprehensionCfg, err := config.LoadComprehensionConfig(filepath.Join(cfg.ConfigDir, "comprehension.yaml"))
	if err != nil {
		logger.WithError(err).Warn("could not load comprehension config")
	}

	comprehension := pipeline.NewComprehensionEngine(comprehensionCfg)

	// Load stopwords for all supported locales (best-effort — empty map on failure).
	allStopwords := osClient.FetchAllStopwords(ctx, supportedLocales)
	logger.WithField("locales", len(allStopwords)).Info("loaded stopwords from linguistic index")

	v1 := pipeline.New(logger, nil,
		pipeline.Normalizer{},
		pipeline.Tokenizer{},
		comprehension,
		pipeline.NewSpellResolver(osClient, pipelineCfg.Spell, logger),
		pipeline.NewSynonymExpander(osClient, logger),
		pipeline.NewCompoundHandler(osClient, logger),
		pipeline.NewStopwordFilter(allStopwords),
		pipeline.NewConceptRecognizer(osClient, pipelineCfg.Concept, logger),
		pipeline.AmbiguityResolver{},
	)

	v3 := native.NewPipeline(native.PipelineConfig{
		FuzzySearcher: osClient,
		Concept:       pipelineCfg.Concept,
		Comprehension: comprehensionCfg,
		Stopwords:     allStopwords,
		Logger:        logger,
	})

	a := &Analyzer{
		v1:     v1,
		v3:     v3,
		logger: logger,
	}

	if cfg.LLM.Enabled {
		v2, err := buildHybridPipeline(ctx, cfg, osClient, comprehension, allStopwords, logger)
		if err != nil {
			return nil, fmt.Errorf("building hybrid pipeline: %w", err)
		}
		a.v2 = v2
	}

	// v4 adaptive pipeline: v3 fast path, optional v2 escalation.
	a.v4 = adaptive.NewPipeline(adaptive.PipelineConfig{
		V3:        v3,
		V2:        a.v2, // nil if LLM not enabled — v4 will fall back to v3
		ScorerCfg: adaptive.ScorerConfigFromYAML(pipelineCfg.Adaptive),
		Logger:    logger,
	})

	return a, nil
}

// Analyze runs the v1 deterministic pipeline.
func (a *Analyzer) Analyze(ctx context.Context, req model.AnalyzeRequest) (model.AnalyzeResponse, error) {
	state := &model.QueryState{
		OriginalQuery:   req.Query,
		NormalizedQuery: req.Query,
		Locale:          req.Locale,
		Market:          req.Market,
	}

	if err := a.v1.Run(ctx, state, false); err != nil {
		return model.AnalyzeResponse{}, err
	}

	return state.ToResponse(), nil
}

// AnalyzeV2 runs the v2 hybrid LLM-augmented pipeline.
// Returns an error if the v2 pipeline was not enabled in config.
func (a *Analyzer) AnalyzeV2(ctx context.Context, req model.AnalyzeRequest) (model.AnalyzeResponse, error) {
	if a.v2 == nil {
		return model.AnalyzeResponse{}, fmt.Errorf("v2 hybrid pipeline is not enabled")
	}
	resp, _ := a.v2.Run(ctx, req, false)
	return resp, nil
}

// AnalyzeV2Debug runs the v2 hybrid pipeline with debug info.
func (a *Analyzer) AnalyzeV2Debug(ctx context.Context, req model.AnalyzeRequest) (model.AnalyzeResponse, any, error) {
	if a.v2 == nil {
		return model.AnalyzeResponse{}, nil, fmt.Errorf("v2 hybrid pipeline is not enabled")
	}
	resp, debugInfo := a.v2.Run(ctx, req, true)
	return resp, debugInfo, nil
}

// AnalyzeV3 runs the v3 native OS-driven pipeline.
func (a *Analyzer) AnalyzeV3(ctx context.Context, req model.AnalyzeRequest) (model.AnalyzeResponse, error) {
	return a.v3.Run(ctx, req)
}

// AnalyzeV4 runs the v4 adaptive pipeline (v3 fast path + v2 LLM escalation).
func (a *Analyzer) AnalyzeV4(ctx context.Context, req model.AnalyzeRequest) (model.AnalyzeResponse, error) {
	result := a.v4.Run(ctx, req)
	return result.Response, nil
}

// HasV2 reports whether the v2 hybrid pipeline is available.
func (a *Analyzer) HasV2() bool {
	return a.v2 != nil
}

func buildHybridPipeline(ctx context.Context, cfg Config, osClient *opensearch.Client, comprehension *pipeline.ComprehensionEngine, stopwords map[string]map[string]bool, logger *logrus.Logger) (*hybrid.Pipeline, error) {
	filtersCfg, err := config.LoadAllowedFiltersConfig(filepath.Join(cfg.ConfigDir, "allowed_filters.yaml"))
	if err != nil {
		return nil, fmt.Errorf("loading allowed filters: %w", err)
	}

	sortsCfg, err := config.LoadAllowedSortsConfig(filepath.Join(cfg.ConfigDir, "allowed_sorts.yaml"))
	if err != nil {
		return nil, fmt.Errorf("loading allowed sorts: %w", err)
	}

	promptBuilder, err := hybrid.NewPromptBuilder(
		filepath.Join(cfg.ConfigDir, "llm_prompt.txt"),
		filtersCfg,
		sortsCfg,
	)
	if err != nil {
		return nil, fmt.Errorf("loading LLM prompt: %w", err)
	}

	bedrockClient, err := bedrock.NewClient(ctx, bedrock.ClientConfig{
		Region:     cfg.LLM.Region,
		ModelID:    cfg.LLM.Model,
		MaxRetries: cfg.LLM.MaxRetries,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating bedrock client: %w", err)
	}

	validator := hybrid.NewValidator(filtersCfg, sortsCfg, cfg.LLM.MinConfidence)
	conceptResolver := hybrid.NewConceptResolver(osClient, logger)
	hybridMetrics := observability.NewHybridMetrics()

	return hybrid.NewPipeline(hybrid.PipelineConfig{
		Parser:          bedrockClient,
		PromptBuilder:   promptBuilder,
		Validator:       validator,
		ConceptResolver: conceptResolver,
		Comprehension:   comprehension,
		Stopwords:       stopwords,
		Metrics:         hybridMetrics,
		Logger:          logger,
		FailOpen:        cfg.LLM.FailOpen,
	}), nil
}
