# QUS — Query Understanding Service

A Go service that parses food search queries into structured intent (filters, sorts, concepts, rewrites). It provides both a deterministic pipeline (v1) and a hybrid LLM-augmented pipeline (v2).

## Architecture

```
┌─────────────┐     ┌─────────────────────────────────────────────────┐
│  /v1/analyze│────▶│ Deterministic Pipeline                          │
│             │     │ Normalize → Tokenize → Spell → Concepts → Sort  │
└─────────────┘     └─────────────────────────────────────────────────┘

┌─────────────┐     ┌─────────────────────────────────────────────────┐
│  /v2/analyze│────▶│ Hybrid Pipeline                                 │
│             │     │ Normalize → Tokenize → LLM Parse → Validate     │
│             │     │           → Resolve Concepts → Assemble          │
└─────────────┘     └─────────────────────────────────────────────────┘
```

The v2 hybrid pipeline uses an LLM (AWS Bedrock or Ollama) as an **advisory** layer — all LLM outputs are schema-validated against allowlists, and the service fails open to deterministic-only results on LLM failure.

## Project Structure

```
pkg/
  model/                      # Public domain types (importable by consumers)
  analyzer/                   # Public API for in-process analysis
  config/                     # Configuration loading (envconfig)
cmd/                          # Cobra CLI commands (http server)
configs/                      # Allowed filters, sorts, LLM prompt
internal/
  application/routes/         # Chi HTTP handlers
  domain/
    pipeline/                 # v1 deterministic pipeline
    hybrid/                   # v2 hybrid LLM pipeline
  infra/
    bedrock/                  # AWS Bedrock Converse API client
    ollama/                   # Ollama local LLM client
    opensearch/               # OpenSearch concept search
    observability/            # Prometheus metrics
```

## Using as a Library

Add QUS as a dependency:

```bash
go get github.com/ahmedalaahagag/query-understanding-service
```

Use the `pkg/analyzer` package for in-process analysis (no HTTP):

```go
import (
    "github.com/ahmedalaahagag/query-understanding-service/pkg/analyzer"
    "github.com/ahmedalaahagag/query-understanding-service/pkg/config"
    "github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

a, err := analyzer.New(ctx, analyzer.Config{
    ConfigDir: "configs",
    OpenSearch: config.OpenSearchConfig{
        URL: "http://localhost:9200",
    },
})

resp, err := a.Analyze(ctx, model.AnalyzeRequest{
    Query:  "cheap chicken recipes",
    Locale: "en-GB",
    Market: "uk",
})
```

Or import just the types to use with the HTTP client:

```go
import "github.com/ahmedalaahagag/query-understanding-service/pkg/model"

var resp model.AnalyzeResponse
```

## Getting Started

### Prerequisites

- Go 1.26+
- Docker & Docker Compose
- AWS credentials (for Bedrock provider)

### Run locally

```bash
cp .env.example env
# Edit env with your settings

docker-compose up -d   # OpenSearch (+ Ollama if using local LLM)
make run
```

### Configuration

All configuration is via environment variables (see `.env.example`):

| Variable | Default | Description |
|---|---|---|
| `QUS_HTTP_PORT` | `8080` | HTTP server port |
| `QUS_OPENSEARCH_URL` | `http://localhost:9200` | OpenSearch endpoint |
| `QUS_LLM_ENABLED` | `false` | Enable v2 hybrid pipeline |
| `QUS_LLM_PROVIDER` | `ollama` | LLM provider: `ollama` or `bedrock` |
| `QUS_LLM_MODEL` | `qwen2.5:3b` | Model ID |
| `QUS_LLM_REGION` | `eu-west-1` | AWS region (Bedrock only) |
| `QUS_LLM_MIN_CONFIDENCE` | `0.65` | Minimum confidence threshold |
| `QUS_LLM_FAIL_OPEN` | `true` | Fall back to deterministic on LLM failure |

## API

### POST /v1/analyze

Deterministic query analysis.

```bash
curl -X POST http://localhost:8080/v1/analyze?locale=en-GB&country=uk \
  -H 'Content-Type: application/json' \
  -d '{"query": "cheap chicken recipes"}'
```

### POST /v2/analyze

Hybrid LLM-augmented analysis (requires `QUS_LLM_ENABLED=true`).

```bash
curl -X POST http://localhost:8080/v2/analyze?locale=en-GB&country=uk \
  -H 'Content-Type: application/json' \
  -d '{"query": "cheap chicken recipes under 500 calories"}'
```

### POST /v2/analyze/debug

Same as v2 but includes debug information (raw LLM output, validation details, timing).

### GET /healthz

Health check endpoint.

### GET /metrics

Prometheus metrics endpoint.

## Development

```bash
make test    # Run all tests with race detector
make lint    # Run golangci-lint
make build   # Build binary to bin/
make fmt     # Format code
```

## Docker

```bash
docker build -t qus .
docker run -p 8080:8080 --env-file env qus
```
