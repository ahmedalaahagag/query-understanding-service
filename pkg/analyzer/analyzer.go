// Package analyzer provides the public API for running query understanding analysis.
// Import this package to use QUS as a library instead of calling it over HTTP.
package analyzer

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/hybrid"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/native"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/pipeline"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/bedrock"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/observability"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/ollama"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
)

// Analyzer runs query understanding analysis in-process.
type Analyzer struct {
	v1     *pipeline.Pipeline
	v2     *hybrid.Pipeline
	v3     *native.Pipeline
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

	synCfg, err := config.LoadSynonymConfig(filepath.Join(cfg.ConfigDir, "synonyms.en-GB.yaml"))
	if err != nil {
		logger.WithError(err).Warn("could not load synonym config")
	}

	compCfg, err := config.LoadCompoundConfig(filepath.Join(cfg.ConfigDir, "compounds.en-GB.yaml"))
	if err != nil {
		logger.WithError(err).Warn("could not load compound config")
	}

	comprehensionCfg, err := config.LoadComprehensionConfig(filepath.Join(cfg.ConfigDir, "comprehension.en-GB.yaml"))
	if err != nil {
		logger.WithError(err).Warn("could not load comprehension config")
	}

	v1 := pipeline.New(logger, nil,
		pipeline.Normalizer{},
		pipeline.Tokenizer{},
		pipeline.NewSpellResolver(osClient, pipelineCfg.Spell, logger),
		pipeline.NewSynonymExpander(osClient, synCfg, logger),
		pipeline.NewCompoundHandler(compCfg),
		pipeline.NewConceptRecognizer(osClient, pipelineCfg.Concept, logger),
		pipeline.AmbiguityResolver{},
		pipeline.NewComprehensionEngine(comprehensionCfg),
	)

	v3 := native.NewPipeline(native.PipelineConfig{
		FuzzySearcher: osClient,
		Concept:       pipelineCfg.Concept,
		Comprehension: comprehensionCfg,
		Logger:        logger,
	})

	a := &Analyzer{
		v1:     v1,
		v3:     v3,
		logger: logger,
	}

	if cfg.LLM.Enabled {
		v2, err := buildHybridPipeline(ctx, cfg, osClient, logger)
		if err != nil {
			return nil, fmt.Errorf("building hybrid pipeline: %w", err)
		}
		a.v2 = v2
	}

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

// HasV2 reports whether the v2 hybrid pipeline is available.
func (a *Analyzer) HasV2() bool {
	return a.v2 != nil
}

func buildHybridPipeline(ctx context.Context, cfg Config, osClient *opensearch.Client, logger *logrus.Logger) (*hybrid.Pipeline, error) {
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

	var parser hybrid.LLMParser
	switch cfg.LLM.Provider {
	case "bedrock":
		bedrockClient, err := bedrock.NewClient(ctx, bedrock.ClientConfig{
			Region:     cfg.LLM.Region,
			ModelID:    cfg.LLM.Model,
			MaxRetries: cfg.LLM.MaxRetries,
		}, logger)
		if err != nil {
			return nil, fmt.Errorf("creating bedrock client: %w", err)
		}
		parser = bedrockClient
	case "ollama":
		parser = ollama.NewClient(ollama.ClientConfig{
			URL:        cfg.LLM.URL,
			Model:      cfg.LLM.Model,
			Timeout:    cfg.LLM.Timeout,
			MaxRetries: cfg.LLM.MaxRetries,
		})
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.LLM.Provider)
	}

	validator := hybrid.NewValidator(filtersCfg, sortsCfg, cfg.LLM.MinConfidence)
	conceptResolver := hybrid.NewConceptResolver(osClient, logger)
	hybridMetrics := observability.NewHybridMetrics()

	return hybrid.NewPipeline(hybrid.PipelineConfig{
		Parser:          parser,
		PromptBuilder:   promptBuilder,
		Validator:       validator,
		ConceptResolver: conceptResolver,
		Metrics:         hybridMetrics,
		Logger:          logger,
		FailOpen:        cfg.LLM.FailOpen,
	}), nil
}
