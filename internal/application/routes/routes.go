package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hellofresh/qus/internal/domain/hybrid"
	"github.com/hellofresh/qus/internal/domain/model"
	"github.com/hellofresh/qus/internal/domain/pipeline"
	"github.com/hellofresh/qus/internal/infra/observability"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// RouterConfig holds all dependencies for creating the router.
type RouterConfig struct {
	Logger         *logrus.Logger
	Pipeline       *pipeline.Pipeline
	Metrics        *observability.Metrics
	HybridPipeline *hybrid.Pipeline
	HybridMetrics  *observability.HybridMetrics
}

// New creates and configures the chi router with all routes.
func New(logger *logrus.Logger, p *pipeline.Pipeline, metrics *observability.Metrics) chi.Router {
	return NewWithConfig(RouterConfig{
		Logger:   logger,
		Pipeline: p,
		Metrics:  metrics,
	})
}

// NewWithConfig creates the router with full config including optional hybrid pipeline.
func NewWithConfig(cfg RouterConfig) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(correlationIDLogger(cfg.Logger))

	r.Get("/healthz", healthHandler())
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Post("/v1/analyze", analyzeHandler(cfg.Logger, cfg.Pipeline, cfg.Metrics))

	if cfg.HybridPipeline != nil {
		r.Post("/v2/analyze", analyzeV2Handler(cfg.Logger, cfg.HybridPipeline, cfg.HybridMetrics))
		r.Post("/v2/analyze/debug", analyzeV2DebugHandler(cfg.Logger, cfg.HybridPipeline, cfg.HybridMetrics))
	}

	return r
}

// correlationIDLogger injects the chi RequestID into the logger for each request.
func correlationIDLogger(logger *logrus.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := middleware.GetReqID(r.Context())
			if reqID != "" {
				logger.WithField("request_id", reqID)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func analyzeHandler(logger *logrus.Logger, p *pipeline.Pipeline, metrics *observability.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if metrics != nil {
				metrics.RequestsTotal.WithLabelValues("bad_request").Inc()
			}
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if locale := r.URL.Query().Get("locale"); locale != "" {
			req.Locale = locale
		}
		if country := r.URL.Query().Get("country"); country != "" {
			req.Market = country
		}

		if req.Query == "" {
			if metrics != nil {
				metrics.RequestsTotal.WithLabelValues("bad_request").Inc()
			}
			http.Error(w, `{"error":"query is required"}`, http.StatusBadRequest)
			return
		}

		debug := r.URL.Query().Get("debug") == "true"

		state := &model.QueryState{
			OriginalQuery:   req.Query,
			NormalizedQuery: req.Query,
			Locale:          req.Locale,
			Market:          req.Market,
		}

		if err := p.Run(r.Context(), state, debug); err != nil {
			if metrics != nil {
				metrics.RequestsTotal.WithLabelValues("error").Inc()
			}
			logger.WithError(err).Error("pipeline execution failed")
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}

		if metrics != nil {
			metrics.RequestsTotal.WithLabelValues("ok").Inc()
		}

		resp := state.ToResponse()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func parseV2Request(r *http.Request) (model.AnalyzeRequest, error) {
	var req model.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}

	// Locale and country from URL query params (override body if present)
	if locale := r.URL.Query().Get("locale"); locale != "" {
		req.Locale = locale
	}
	if country := r.URL.Query().Get("country"); country != "" {
		req.Market = country
	}

	return req, nil
}

func analyzeV2Handler(logger *logrus.Logger, hp *hybrid.Pipeline, metrics *observability.HybridMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := parseV2Request(r)
		if err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Query == "" {
			http.Error(w, `{"error":"query is required"}`, http.StatusBadRequest)
			return
		}

		resp, _ := hp.Run(r.Context(), req, false)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func analyzeV2DebugHandler(logger *logrus.Logger, hp *hybrid.Pipeline, metrics *observability.HybridMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := parseV2Request(r)
		if err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Query == "" {
			http.Error(w, `{"error":"query is required"}`, http.StatusBadRequest)
			return
		}

		resp, debugInfo := hp.Run(r.Context(), req, true)

		result := struct {
			Response model.AnalyzeResponse `json:"response"`
			Debug    *hybrid.DebugInfo     `json:"debug,omitempty"`
		}{
			Response: resp,
			Debug:    debugInfo,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
