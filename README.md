# QUS — Query Understanding Service

A Go library and HTTP service that parses search queries into structured intent: normalized tokens, concept matches, filters, sorts, and rewrites. It provides three pipelines: deterministic (v1), LLM-augmented hybrid (v2), and native OpenSearch-driven (v3).

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

┌─────────────┐     ┌─────────────────────────────────────────────────┐
│  /v3/analyze│────▶│ Native OS Pipeline                              │
│             │     │ Normalize → Tokenize → OS Fuzzy Spell           │
│             │     │           → OS Fuzzy Concepts → Comprehension    │
└─────────────┘     └─────────────────────────────────────────────────┘
```

**v1** handles synonyms, compounds, and spell correction in Go application code using YAML configs.

**v2** uses an LLM (AWS Bedrock or Ollama) as an **advisory** layer. All LLM outputs are schema-validated against allowlists, and the service fails open to deterministic-only results on LLM failure.

**v3** delegates spell correction, synonym matching, and compound handling to OpenSearch natively via `fuzziness: AUTO` + `cross_fields` multi_match. No YAML synonym/compound configs needed — OpenSearch handles it.

## Project Structure

```
pkg/
  model/                      # Public domain types (importable by consumers)
  analyzer/                   # Public API for in-process analysis
  config/                     # Configuration types and loaders
cmd/                          # Cobra CLI (HTTP server mode)
configs/                      # YAML config files and LLM prompt
internal/
  application/routes/         # Chi HTTP handlers
  domain/
    pipeline/                 # v1 deterministic pipeline steps
    hybrid/                   # v2 hybrid LLM pipeline
    native/                   # v3 native OS-driven pipeline
  infra/
    bedrock/                  # AWS Bedrock Converse API client
    ollama/                   # Ollama local LLM client
    opensearch/               # OpenSearch concept/spell/linguistic search
    observability/            # Prometheus metrics
```

---

## How to Use

### Installation

```bash
go get github.com/ahmedalaahagag/query-understanding-service
```

### Option A: Use as a Go Library (in-process)

Import the `pkg/analyzer` package to run query analysis directly in your service — no HTTP calls or separate deployment needed.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/ahmedalaahagag/query-understanding-service/pkg/analyzer"
    "github.com/ahmedalaahagag/query-understanding-service/pkg/config"
    "github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

func main() {
    ctx := context.Background()

    a, err := analyzer.New(ctx, analyzer.Config{
        ConfigDir: "configs",
        OpenSearch: config.OpenSearchConfig{
            URL: "http://localhost:9200",
        },
        // Optional: enable v2 hybrid LLM pipeline
        // LLM: config.LLMConfig{
        //     Enabled:  true,
        //     Provider: "bedrock",
        //     Region:   "eu-west-1",
        //     Model:    "eu.amazon.nova-micro-v1:0",
        // },
    })
    if err != nil {
        log.Fatal(err)
    }

    // v1: deterministic analysis
    resp, err := a.Analyze(ctx, model.AnalyzeRequest{
        Query:  "cheap chicken recipes",
        Locale: "en-GB",
        Market: "uk",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Normalized: %s\n", resp.NormalizedQuery)
    fmt.Printf("Tokens:     %d\n", len(resp.Tokens))
    fmt.Printf("Concepts:   %d\n", len(resp.Concepts))
    fmt.Printf("Filters:    %d\n", len(resp.Filters))

    // v3: native OS pipeline (always available, no extra config)
    resp, err = a.AnalyzeV3(ctx, model.AnalyzeRequest{
        Query:  "chiken soup",  // typo corrected via OS fuzzy matching
        Locale: "en-GB",
        Market: "uk",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("V3 Normalized: %s\n", resp.NormalizedQuery)

    // v2: hybrid LLM analysis (only if LLM is enabled)
    if a.HasV2() {
        resp, err := a.AnalyzeV2(ctx, model.AnalyzeRequest{
            Query:  "chicken under 500 calories",
            Locale: "en-GB",
            Market: "uk",
        })
        if err != nil {
            log.Fatal(err)
        }
        fmt.Printf("V2 Filters: %+v\n", resp.Filters)
    }
}
```

### Option B: Import Types Only

If you prefer to call QUS over HTTP but want shared types (no duplication):

```go
import "github.com/ahmedalaahagag/query-understanding-service/pkg/model"

// Use these in your HTTP client
var req model.AnalyzeRequest
var resp model.AnalyzeResponse
```

### Option C: Run as a Standalone HTTP Service

```bash
cp .env.example env
# Edit env with your settings

docker-compose up -d   # Start OpenSearch
make run               # Start QUS HTTP server on :8080
```

---

## Configuration Files

QUS loads configuration from a directory (default: `configs/`). Each file serves a specific purpose.

### Required Files

#### `qus.yaml` — Pipeline Settings

Controls the deterministic pipeline behavior.

```yaml
pipeline:
  enabled_steps:
    - normalize
    - tokenize
    - spell
    - synonym
    - compound
    - concept
    - ambiguity
    - comprehension

spell:
  enabled: true
  min_token_length: 4          # Skip spell-check for short tokens
  confidence_threshold: 0.85   # Minimum score to accept a correction

concept:
  shingle_max_size: 4          # Max tokens in a concept span (e.g. "peanut butter" = 2)
  max_matches_per_span: 3      # Max concept candidates per token span

ambiguity:
  prefer_longest_span: true    # Prefer "ice cream" over "ice" + "cream"
  min_score_delta: 0.05        # Score gap needed to pick one concept over another
```

#### `comprehension.{locale}.yaml` — Filter & Sort Extraction Rules

Regex-based rules for extracting filters and sort directives from natural language.

```yaml
price_rules:
  - pattern: '(under|less than|cheaper than)\s+(\d+(?:\.\d+)?)'
    field: price
    operator: lt

sort_rules:
  - pattern: '(cheapest|lowest price)'
    field: price
    direction: asc
  - pattern: '(newest|most recent)'
    field: created_at
    direction: desc
```

### Config Files per Pipeline

| Config File | v1 | v2 | v3 |
|---|---|---|---|
| `qus.yaml` | ✅ | — | ✅ |
| `comprehension.{locale}.yaml` | ✅ | — | ✅ |
| `allowed_filters.yaml` | — | ✅ | — |
| `allowed_sorts.yaml` | — | ✅ | — |
| `llm_prompt.txt` | — | ✅ | — |

Synonyms and compounds are stored in OpenSearch's linguistic index (types `SYN`, `HYP`, `CMP`) — no YAML data files needed.

### V2 Hybrid Pipeline Files (optional)

These are only needed when `LLM.Enabled = true`.

#### `allowed_filters.yaml` — Filter Allowlist

Defines which filters the LLM is allowed to produce. Any LLM-suggested filter not in this list is silently dropped.

```yaml
filters:
  - field: price
    operators: [lt, lte, gt, gte, eq]
    type: number

  - field: dietary
    operators: [eq, in]
    type: string_list

  - field: cuisine
    operators: [eq, in]
    type: string_list

  - field: calories
    operators: [lt, lte, gt, gte, eq]
    type: number
```

#### `allowed_sorts.yaml` — Sort Allowlist

Defines which sort keys the LLM is allowed to produce.

```yaml
sorts:
  - key: relevance
  - key: price_asc
  - key: price_desc
  - key: newest
  - key: prep_time_asc
  - key: calories_asc
```

#### `llm_prompt.txt` — LLM System Prompt

The system prompt sent to the LLM. The allowed filters and sorts are automatically appended at runtime. Keep it compact for low latency.

```
Parse food search query into JSON. Raw JSON only, no markdown.
{"normalizedQuery":"","rewrites":[],"candidateConcepts":[{"label":"","field":"","confidence":0.0}],"filters":[{"field":"","operator":"","value":null,"confidence":0.0}],"sort":{"field":"","direction":"asc|desc","confidence":0.0},"confidence":0.0,"warnings":[]}
Rules: only use ALLOWED FILTERS/SORTS below. Fix typos. Empty arrays if uncertain. Max 1 rewrite.
```

---

## OpenSearch Indexes

QUS expects two OpenSearch indexes per locale. Use the seed script to populate them.

### Concept Index (`concepts_{locale}`)

Stores known entities (categories, ingredients, tags) that QUS can recognize in queries.

```json
{
  "id": "uk-cat-chicken",
  "label": "chicken",
  "field": "category",
  "aliases": ["poultry"],
  "weight": 10,
  "locale": "en_GB",
  "market": "UK"
}
```

### Linguistic Index (`linguistic_{locale}`)

Stores synonym/hypernym relationships for spell-correction and expansion.

```json
{
  "term": "capsicum",
  "variant": "bell pepper",
  "type": "SYN",
  "locale": "en_AU"
}
```

Type values: `SYN` (synonym), `HYP` (hypernym), `CMP` (compound), `SW` (stopword).

**Compound example** — `term` is the joined form, `variant` is the split form:

```json
{
  "term": "icecream",
  "variant": "ice cream",
  "type": "CMP",
  "locale": "en_GB"
}
```

### Seeding

```bash
# Seed all locale data (concepts + linguistic + compounds)
./scripts/seed-opensearch.sh

# Seed compound rules only (from scripts/compound-data/*.tsv)
./scripts/seed-compounds.sh

# Seed compounds for a single locale
./scripts/seed-compounds.sh http://localhost:9200 en_gb
```

Sample data is provided in `scripts/locale-data/` for 20+ locales and `scripts/compound-data/` for compound word rules.

#### Adding Compound Words

Add entries to `scripts/compound-data/{locale}.tsv` (tab-separated):

```
icecream	ice cream
peanutbutter	peanut butter
```

Then run `./scripts/seed-compounds.sh` to index them.

---

## Environment Variables

For standalone HTTP mode, configure via environment (see `.env.example`):

| Variable | Default | Description |
|---|---|---|
| `QUS_HTTP_PORT` | `8080` | HTTP server port |
| `QUS_OPENSEARCH_URL` | `http://localhost:9200` | OpenSearch endpoint |
| `QUS_OPENSEARCH_USERNAME` | | OpenSearch basic auth username |
| `QUS_OPENSEARCH_PASSWORD` | | OpenSearch basic auth password |
| `QUS_OPENSEARCH_CONCEPT_INDEX_PREFIX` | `concepts` | Concept index prefix |
| `QUS_OPENSEARCH_LINGUISTIC_INDEX_PREFIX` | `linguistic` | Linguistic index prefix |
| `QUS_CONFIG_DIR` | `configs` | Path to config files directory |
| `QUS_LLM_ENABLED` | `false` | Enable v2 hybrid pipeline |
| `QUS_LLM_PROVIDER` | `ollama` | LLM provider: `ollama` or `bedrock` |
| `QUS_LLM_MODEL` | `qwen2.5:3b` | Model ID |
| `QUS_LLM_URL` | `http://localhost:11434` | Ollama URL (ollama only) |
| `QUS_LLM_REGION` | `eu-west-1` | AWS region (bedrock only) |
| `QUS_LLM_TIMEOUT` | `30s` | LLM request timeout |
| `QUS_LLM_MAX_RETRIES` | `1` | Retry count on LLM failure |
| `QUS_LLM_MIN_CONFIDENCE` | `0.65` | Minimum confidence threshold |
| `QUS_LLM_FAIL_OPEN` | `true` | Fall back to deterministic on LLM failure |

---

## REST API

### POST /v1/analyze

Deterministic query analysis.

**Query parameters:** `locale`, `country`

```bash
curl -X POST 'http://localhost:8080/v1/analyze?locale=en-GB&country=uk' \
  -H 'Content-Type: application/json' \
  -d '{"query": "cheap chicken recipes"}'
```

**Response:**

```json
{
  "originalQuery": "cheap chicken recipes",
  "normalizedQuery": "cheap chicken recipes",
  "tokens": [
    {"value": "cheap", "normalized": "cheap", "position": 0},
    {"value": "chicken", "normalized": "chicken", "position": 1},
    {"value": "recipes", "normalized": "recipes", "position": 2}
  ],
  "rewrites": [],
  "concepts": [
    {"id": "uk-cat-chicken", "label": "chicken", "matchedText": "chicken", "field": "category", "score": 0.95, "source": "exact", "start": 1, "end": 1}
  ],
  "filters": [],
  "sort": {"field": "price", "direction": "asc"},
  "warnings": []
}
```

### POST /v2/analyze

Hybrid LLM-augmented analysis (requires `QUS_LLM_ENABLED=true`).

**Query parameters:** `locale`, `country`

```bash
curl -X POST 'http://localhost:8080/v2/analyze?locale=en-GB&country=uk' \
  -H 'Content-Type: application/json' \
  -d '{"query": "chicken under 500 calories"}'
```

### POST /v3/analyze

Native OpenSearch-driven analysis. Delegates spell correction and concept matching to OpenSearch via `fuzziness: AUTO` + `cross_fields` multi_match. No YAML synonym/compound configs needed.

**Query parameters:** `locale`, `country`

```bash
curl -X POST 'http://localhost:8080/v3/analyze?locale=en-GB&country=uk' \
  -H 'Content-Type: application/json' \
  -d '{"query": "chiken soup under 5"}'
```

### POST /v2/analyze/debug

Same as v2, returns additional debug info (raw LLM output, validation details, timing).

### GET /healthz

Returns `{"status": "ok"}` with HTTP 200.

### GET /metrics

Prometheus metrics endpoint.

---

## Response Types Reference

### `model.AnalyzeResponse`

| Field | Type | Description |
|---|---|---|
| `originalQuery` | `string` | The raw input query |
| `normalizedQuery` | `string` | Lowercased, trimmed, whitespace-collapsed query |
| `tokens` | `[]Token` | Tokenized query terms |
| `rewrites` | `[]string` | Alternative query forms (spell corrections, synonyms) |
| `concepts` | `[]ConceptMatch` | Recognized entities matched against OpenSearch |
| `filters` | `[]Filter` | Extracted filter directives (e.g. price < 10) |
| `sort` | `*SortSpec` | Extracted sort directive (e.g. cheapest → price asc) |
| `warnings` | `[]string` | Non-fatal issues (unresolved concepts, validation drops) |

### `model.Token`

| Field | Type | Description |
|---|---|---|
| `value` | `string` | Original token text |
| `normalized` | `string` | Lowercased/cleaned token |
| `position` | `int` | Zero-based position in query |

### `model.ConceptMatch`

| Field | Type | Description |
|---|---|---|
| `id` | `string` | OpenSearch document ID |
| `label` | `string` | Canonical concept label |
| `matchedText` | `string` | Text in the query that matched |
| `field` | `string` | Index field (e.g. `category`, `ingredient`) |
| `score` | `float64` | Match confidence score |
| `source` | `string` | How it was matched: `exact`, `alias`, `fuzzy`, or `llm` |
| `start` | `int` | Start token position |
| `end` | `int` | End token position |

### `model.Filter`

| Field | Type | Description |
|---|---|---|
| `field` | `string` | Filter field name (e.g. `price`, `calories`) |
| `operator` | `string` | Comparison operator: `eq`, `lt`, `lte`, `gt`, `gte`, `in` |
| `value` | `any` | Filter value (number, string, or list) |

### `model.SortSpec`

| Field | Type | Description |
|---|---|---|
| `field` | `string` | Sort field name |
| `direction` | `string` | `asc` or `desc` |

---

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

Docker Compose brings up OpenSearch and optionally Ollama for local LLM:

```bash
docker-compose up -d
```
