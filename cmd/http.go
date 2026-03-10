package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"path/filepath"

	"github.com/hellofresh/qus/internal/application/routes"
	"github.com/hellofresh/qus/internal/domain/hybrid"
	"github.com/hellofresh/qus/internal/domain/pipeline"
	"github.com/hellofresh/qus/internal/infra/bedrock"
	"github.com/hellofresh/qus/internal/infra/observability"
	"github.com/hellofresh/qus/internal/infra/ollama"
	"github.com/hellofresh/qus/internal/infra/opensearch"
	"github.com/hellofresh/qus/pkg/config"
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
		"config_dir":    cfg.ConfigDir,
		"opensearch":    cfg.OpenSearch.URL,
		"port":          cfg.HTTP.Port,
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

	p := pipeline.New(logger, metrics,
		pipeline.Normalizer{},
		pipeline.Tokenizer{},
		pipeline.NewSpellResolver(osClient, pipelineCfg.Spell, logger),
		pipeline.NewSynonymExpander(osClient, synCfg, logger),
		pipeline.NewCompoundHandler(compCfg),
		pipeline.NewConceptRecognizer(osClient, pipelineCfg.Concept, logger),
		pipeline.AmbiguityResolver{},
		pipeline.NewComprehensionEngine(comprehensionCfg),
	)

	routerCfg := routes.RouterConfig{
		Logger:   logger,
		Pipeline: p,
		Metrics:  metrics,
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
