package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ahmedalaahagag/query-understanding-service/internal/application/routes"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/adaptive"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/hybrid"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/native"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/pipeline"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/bedrock"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/observability"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/ollama"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newHTTPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "http",
		Short: "Start the HTTP server",
		RunE:  runHTTP,
	}
}

func runHTTP(cmd *cobra.Command, args []string) error {
	logger := observability.NewLogger()

	cfg, err := config.Load("QUS")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"config_dir": cfg.ConfigDir,
		"opensearch": cfg.OpenSearch.URL,
		"port":       cfg.HTTP.Port,
	}).Info("configuration loaded")

	metrics := observability.NewMetrics()

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

	osClient := opensearch.NewClient(opensearch.ClientConfig{
		URL:                   cfg.OpenSearch.URL,
		Username:              cfg.OpenSearch.Username,
		Password:              cfg.OpenSearch.Password,
		ConceptIndexPrefix:    cfg.OpenSearch.ConceptIndexPrefix,
		LinguisticIndexPrefix: cfg.OpenSearch.LinguisticIndexPrefix,
		Timeout:               cfg.OpenSearch.Timeout,
	})

	comprehensionCfg, err := config.LoadComprehensionConfig(filepath.Join(cfg.ConfigDir, "comprehension.yaml"))
	if err != nil {
		logger.WithError(err).Warn("could not load comprehension config")
	}

	// Load stopwords for all supported locales (best-effort).
	allLocales := []string{
		"en_gb", "en_us", "en_ca", "en_au", "en_ie", "en_nz",
		"en_be", "en_de", "en_dk", "en_nl", "en_se",
		"de_de", "de_at", "de_ch",
		"fr_fr", "fr_ca", "fr_be", "fr_lu",
		"nl_nl", "nl_be",
		"it_it", "es_es", "sv_se", "da_dk", "nb_no", "ja_jp",
	}
	allStopwords := osClient.FetchAllStopwords(context.Background(), allLocales)
	logger.WithField("locales", len(allStopwords)).Info("loaded stopwords from linguistic index")

	p := pipeline.New(logger, metrics,
		pipeline.Normalizer{},
		pipeline.Tokenizer{},
		pipeline.NewComprehensionEngine(comprehensionCfg),
		pipeline.NewSpellResolver(osClient, pipelineCfg.Spell, logger),
		pipeline.NewSynonymExpander(osClient, logger),
		pipeline.NewCompoundHandler(osClient, logger),
		pipeline.NewStopwordFilter(allStopwords),
		pipeline.NewConceptRecognizer(osClient, pipelineCfg.Concept, logger),
		pipeline.AmbiguityResolver{},
	)

	// v3 native pipeline — uses OS fuzzy matching for spell/synonym/compound
	nativePipeline := native.NewPipeline(native.PipelineConfig{
		FuzzySearcher: osClient,
		Concept:       pipelineCfg.Concept,
		Comprehension: comprehensionCfg,
		Stopwords:     allStopwords,
		Logger:        logger,
	})

	routerCfg := routes.RouterConfig{
		Logger:         logger,
		Pipeline:       p,
		Metrics:        metrics,
		NativePipeline: nativePipeline,
	}

	if cfg.LLM.Enabled {
		hybridMetrics := observability.NewHybridMetrics()

		filtersCfg, err := config.LoadAllowedFiltersConfig(filepath.Join(cfg.ConfigDir, "allowed_filters.yaml"))
		if err != nil {
			return fmt.Errorf("loading allowed filters config: %w", err)
		}

		sortsCfg, err := config.LoadAllowedSortsConfig(filepath.Join(cfg.ConfigDir, "allowed_sorts.yaml"))
		if err != nil {
			return fmt.Errorf("loading allowed sorts config: %w", err)
		}

		promptBuilder, err := hybrid.NewPromptBuilder(
			filepath.Join(cfg.ConfigDir, "llm_prompt.txt"),
			filtersCfg,
			sortsCfg,
		)
		if err != nil {
			return fmt.Errorf("loading LLM prompt: %w", err)
		}

		var parser hybrid.LLMParser
		switch cfg.LLM.Provider {
		case "bedrock":
			bedrockClient, err := bedrock.NewClient(context.Background(), bedrock.ClientConfig{
				Region:     cfg.LLM.Region,
				ModelID:    cfg.LLM.Model,
				MaxRetries: cfg.LLM.MaxRetries,
			}, logger)
			if err != nil {
				return fmt.Errorf("creating bedrock client: %w", err)
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
			return fmt.Errorf("unknown LLM provider: %s (must be ollama or bedrock)", cfg.LLM.Provider)
		}

		validator := hybrid.NewValidator(filtersCfg, sortsCfg, cfg.LLM.MinConfidence)
		conceptResolver := hybrid.NewConceptResolver(osClient, logger)

		hybridPipeline := hybrid.NewPipeline(hybrid.PipelineConfig{
			Parser:          parser,
			PromptBuilder:   promptBuilder,
			Validator:       validator,
			ConceptResolver: conceptResolver,
			Comprehension:   pipeline.NewComprehensionEngine(comprehensionCfg),
			Stopwords:       allStopwords,
			Metrics:         hybridMetrics,
			Logger:          logger,
			FailOpen:        cfg.LLM.FailOpen,
		})

		routerCfg.HybridPipeline = hybridPipeline
		routerCfg.HybridMetrics = hybridMetrics

		logger.WithFields(logrus.Fields{
			"llm_provider": cfg.LLM.Provider,
			"llm_model":    cfg.LLM.Model,
		}).Info("hybrid LLM pipeline enabled")
	}

	// v4 adaptive pipeline: v3 fast path, optional v2 LLM escalation.
	routerCfg.AdaptivePipeline = adaptive.NewPipeline(adaptive.PipelineConfig{
		V3:        nativePipeline,
		V2:        routerCfg.HybridPipeline,
		ScorerCfg: adaptive.ScorerConfigFromYAML(pipelineCfg.Adaptive),
		Logger:    logger,
	})

	router := routes.NewWithConfig(routerCfg)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.WithField("port", cfg.HTTP.Port).Info("starting HTTP server")
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.WithField("signal", sig.String()).Info("shutting down")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	logger.Info("server stopped")
	return nil
}
