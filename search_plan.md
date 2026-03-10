# MVP Plan for Rebuilding the Search Orchestrator in Go

## Objective

Build an **MVP Search Orchestrator** in Go that sits between clients and OpenSearch, consumes QUS output, executes deterministic search strategies, and returns clean search results with filters, facets, pagination, and fallback behavior.

The MVP should support:

* request normalization into a search execution plan
* integration with QUS output
* default filters
* structured OpenSearch query building
* staged search fallback
* faceting
* pagination
* sorting
* result transformation
* observability and golden tests

The MVP should **not** attempt to port the full Java workflow engine, chain-of-responsibility tree, or merch-rule behavior.

---

# 1. Product Goal

The Search Orchestrator exists to translate user search intent into a controlled OpenSearch execution flow.

### Input

```json
{
  "query": "cheap chicken burger",
  "locale": "en-GB",
  "market": "uk",
  "page": {
    "size": 24,
    "cursor": null
  },
  "filters": {
    "category": ["burgers"],
    "availability": ["in_stock"]
  },
  "sort": "relevance",
  "userContext": {
    "customerId": "123",
    "segment": "default"
  }
}
```

### QUS-enriched internal input

```json
{
  "originalQuery": "cheap chiken burger",
  "normalizedQuery": "cheap chicken burger",
  "rewrites": ["cheap chicken burger"],
  "concepts": [
    {
      "id": "product_type_burger",
      "label": "burger",
      "matchedText": "burger",
      "field": "product_type",
      "score": 0.98,
      "source": "exact",
      "start": 2,
      "end": 2
    }
  ],
  "filters": [
    {
      "field": "price",
      "operator": "lt",
      "value": 20
    }
  ],
  "sort": {
    "field": "price",
    "direction": "asc"
  }
}
```

### Output

```json
{
  "searchId": "8c45d3d6-f4e2-4da2-9f8a-d2f1c95b9a7e",
  "query": "cheap chicken burger",
  "appliedStage": "fallback_partial",
  "pageInfo": {
    "size": 24,
    "nextCursor": "eyJzZWFyY2hfYWZ0ZXIiOlsuLi5dfQ==",
    "hasNextPage": true
  },
  "items": [
    {
      "id": "sku_123",
      "title": "Chicken Burger",
      "score": 12.83,
      "availability": "in_stock",
      "price": 9.99
    }
  ],
  "facets": [
    {
      "field": "category",
      "values": [
        { "value": "burgers", "count": 124 },
        { "value": "wraps", "count": 18 }
      ]
    }
  ],
  "debug": {
    "stagesTried": ["exact", "fallback_partial"],
    "totalHits": 215
  }
}
```

This output becomes the contract for clients.

---

# 2. MVP Scope

## In scope

### Phase 1 core features

* HTTP API
* QUS integration through interface
* request validation
* search plan building
* default filter application
* OpenSearch query generation
* staged search fallback
* exact + partial retrieval strategy
* faceting
* sorting
* cursor pagination using `search_after`
* result mapping
* structured JSON response
* metrics / logging / tracing hooks
* unit, integration, and golden tests

## Out of scope

* merchandising rules
* boosting/pinning/burying from admin systems
* query redirects
* personalization/reranking engines
* vector search in phase 1 unless explicitly needed
* business-owned workflow UIs
* full chain-of-responsibility parity from Java
* dynamic runtime workflow mutation
* result diversity overrides
* complex RBE-style post processing

---

# 3. Architecture

## Target service shape

```text
Client
  │
  ▼
Search API (Go)
  │
  ├── Request Validator
  ├── QUS Client
  ├── Search Planner
  ├── Query Builder
  ├── Stage Executor
  ├── Facet Builder
  ├── Result Mapper
  │
  ▼
OpenSearch
```

## Internal execution flow

```text
Request
  → Validate
  → Call QUS
  → Build Search Plan
  → Apply Default Filters
  → Execute Stage 1
  → Check result threshold
  → Execute fallback stage if needed
  → Build facets
  → Map response
  → Return
```

---

# 4. Design Principles

1. **Plan first, execute second**

   * construct a deterministic search plan before calling OpenSearch

2. **Keep stages explicit**

   * no magic workflow engine in MVP

3. **Use OpenSearch natively**

   * avoid adapter abstractions that mimic Java unnecessarily

4. **Single canonical orchestration path**

   * avoid runtime-pluggable chains for MVP

5. **Facets and hits must be consistent**

   * define clearly when facet counts exclude self-filters

6. **Golden tests define behavior**

   * especially fallback and filtering behavior

---

# 5. Recommended Repository Layout

```text
search-orchestrator/
├── cmd/search-orchestrator/
│   └── main.go
├── internal/
│   ├── api/
│   ├── app/
│   ├── config/
│   ├── model/
│   ├── planner/
│   ├── qus/
│   ├── defaults/
│   ├── builder/
│   ├── executor/
│   ├── facets/
│   ├── pagination/
│   ├── mapper/
│   ├── opensearch/
│   ├── metrics/
│   └── testutil/
├── configs/
│   ├── search.yaml
│   ├── defaults.yaml
│   ├── facets.yaml
│   └── boosts.yaml
├── testdata/
│   ├── golden/
│   └── fixtures/
├── Makefile
├── go.mod
└── README.md
```

---

# 6. Domain Model

## Request model

```go
type SearchRequest struct {
    Query       string                 `json:"query"`
    Locale      string                 `json:"locale"`
    Market      string                 `json:"market"`
    Filters     map[string][]string    `json:"filters,omitempty"`
    Sort        string                 `json:"sort,omitempty"`
    Page        PageRequest            `json:"page"`
    UserContext map[string]string      `json:"userContext,omitempty"`
}
```

## Pagination

```go
type PageRequest struct {
    Size   int     `json:"size"`
    Cursor *string `json:"cursor,omitempty"`
}
```

## Search plan

```go
type SearchPlan struct {
    Query            string
    NormalizedQuery  string
    Rewrites         []string
    Concepts         []ConceptMatch
    AppliedFilters   []Filter
    AppliedSort      SortSpec
    Stages           []SearchStage
    FacetRequests    []FacetRequest
    Page             PageRequest
}
```

## Stage definition

```go
type SearchStage struct {
    Name              string
    QueryMode         string
    MinimumHits       int
    MinimumScore      *float64
    UseFallbackTerms  bool
}
```

## Response model

```go
type SearchResponse struct {
    SearchID    string         `json:"searchId"`
    Query       string         `json:"query"`
    AppliedStage string        `json:"appliedStage"`
    PageInfo    PageInfo       `json:"pageInfo"`
    Items       []SearchItem   `json:"items"`
    Facets      []FacetResult  `json:"facets,omitempty"`
    Debug       *DebugInfo     `json:"debug,omitempty"`
}
```

---

# 7. Pipeline Definition

## Ordered pipeline for MVP

### Step 1 — Validate request

Tasks:

* ensure query is present unless browse mode supported
* validate locale/market
* validate page size bounds
* validate sort value against allowed list

### Step 2 — Call QUS

Tasks:

* send query, locale, market
* retrieve normalized query, concepts, extracted filters, sort
* fail safely if QUS is unavailable according to chosen fallback mode

### Step 3 — Merge request filters with QUS filters

Tasks:

* combine client filters and QUS-derived filters
* resolve conflicts deterministically
* preserve explicit client filters over inferred filters where needed

### Step 4 — Apply default filters

Tasks:

* availability
* hidden=false
* market/locale restrictions
* catalog constraints

### Step 5 — Build search plan

Tasks:

* determine stage list
* choose exact-first then partial fallback
* attach facet configuration
* select sort behavior

### Step 6 — Execute stage 1

Tasks:

* build OpenSearch DSL
* run hits query
* optionally run facets in same request if supported by plan
* inspect hit count / score threshold

### Step 7 — Execute fallback stage if needed

Tasks:

* broaden match behavior
* use lower `minimum_should_match`
* optionally reduce strict phrase conditions

### Step 8 — Build facets

Tasks:

* compute aggregations
* exclude self-filter for each facet if required

### Step 9 — Map results

Tasks:

* map OpenSearch hits to API schema
* build cursor from `search_after`
* include stage debug metadata

---

# 8. Search Strategy

## MVP stage model

Use only two stages first.

### Stage 1 — Exact / high-confidence relevance

Characteristics:

* stronger field boosts
* stricter matching
* exact concept filters where appropriate
* tighter phrase / full-term expectations

### Stage 2 — Partial fallback

Characteristics:

* lower matching threshold
* broader multi-match
* typo-tolerant fallback if relevant
* lower precision, higher recall

## Example logic

```text
Stage 1:
- exact title / keyword boosted
- exact concept field matches boosted
- minimum_should_match stricter

If hits < threshold:
Stage 2:
- broad multi_match
- lower minimum_should_match
- allow fuzzier retrieval
```

---

# 9. OpenSearch Query Shape

## Stage 1

Use a `bool` with:

* `filter`
* `must` or `should` depending on mode
* field boosts in `multi_match`
* exact keyword boosts where useful

Example components:

* title^5
* subtitle^2
* brand^3
* concept-aligned fields^4

## Stage 2

Use broader `multi_match` with:

* `best_fields` or `cross_fields`
* lower `minimum_should_match`
* fuzziness if safe
* more tolerant term coverage

## Filters

Use filter clauses for:

* market
* locale
* hidden status
* availability
* explicit user filters
* QUS-derived structured filters

---

# 10. Faceting Strategy

## MVP faceting requirements

Support:

* category
* brand
* price range
* dietary tags
* availability

## Principle

Facet counts should usually exclude the facet’s own active filter.

Example:

If user selects `category=burgers`, the `category` facet should still show other category counts based on the rest of the filter set.

This requires per-facet filter rewriting.

## Facet config example

```yaml
facets:
  - field: category
    type: terms
    size: 20
    exclude_self: true

  - field: brand
    type: terms
    size: 20
    exclude_self: true

  - field: price
    type: range
    exclude_self: true
    ranges:
      - key: "0-10"
        from: 0
        to: 10
      - key: "10-20"
        from: 10
        to: 20
```

---

# 11. Sorting Strategy

Support only a small allowed set in MVP:

* `relevance`
* `price_asc`
* `price_desc`
* `newest`

## Sorting rules

### Relevance

* use `_score`
* add stable tiebreaker field such as `id.keyword`

### Price sorts

* explicit sort field
* stable tiebreaker
* `_score` optional secondary if needed

### Newest

* publish date desc
* stable tiebreaker

Do not accept arbitrary field sorting in MVP.

---

# 12. Pagination Strategy

Use `search_after`, not offset pagination for deeper pages.

## Cursor model

Base64-encoded JSON structure:

```json
{
  "search_after": [12.83, "sku_123"]
}
```

## Rules

* cursor tied to sort order
* cursor invalid if sort changes
* cursor invalid if query/filter changes

For MVP, no bidirectional pagination required.

---

# 13. QUS Integration

## Interface

```go
type QUSClient interface {
    Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error)
}
```

## Fallback behavior

If QUS fails:

### Option A — hard fail

Only use if QUS is mandatory.

### Option B — degrade gracefully

Recommended MVP behavior:

* use raw query
* apply explicit request filters only
* skip concepts and inferred filters
* return warning/debug flag

This is safer operationally.

---

# 14. Config Files

## Main config

```yaml
server:
  port: 8081

search:
  index: products
  default_page_size: 24
  max_page_size: 100

stages:
  - name: exact
    minimum_hits: 12
    query_mode: exact
    use_fallback_terms: false

  - name: fallback_partial
    minimum_hits: 1
    query_mode: partial
    use_fallback_terms: true

qus:
  enabled: true
  timeout_ms: 150

opensearch:
  timeout_ms: 300
```

## Default filters

```yaml
default_filters:
  - field: hidden
    operator: eq
    value: false
  - field: availability
    operator: in
    value: ["in_stock"]
```

## Sort config

```yaml
sorts:
  relevance:
    fields:
      - field: "_score"
        direction: "desc"
      - field: "id.keyword"
        direction: "asc"

  price_asc:
    fields:
      - field: "price"
        direction: "asc"
      - field: "id.keyword"
        direction: "asc"
```

---

# 15. API Endpoints

## Health

```http
GET /healthz
```

## Search

```http
POST /v1/search
Content-Type: application/json
```

## Optional debug endpoint

```http
POST /v1/search/debug
```

Returns:

* built search plan
* OpenSearch DSL
* stages tried
* QUS output

Keep behind config flag.

---

# 16. Step-by-Step Delivery Plan for Claude Code

## Phase 0 — Bootstrap

### Goal

Create runnable search service skeleton.

### Tasks

1. Initialize Go module
2. Add HTTP server
3. Add config loader
4. Add `/healthz`
5. Add `/v1/search`
6. Define request/response structs
7. Add structured logging
8. Return stubbed response

### Done criteria

* service runs
* search endpoint accepts request and returns deterministic stub

---

## Phase 1 — Request validation and response model

### Goal

Establish stable API contract.

### Tasks

1. Validate page size
2. Validate sort enum
3. Validate locale/market required fields
4. Add response schema for page info, items, facets
5. Add unit tests for validation

### Done criteria

* invalid requests rejected deterministically
* response contract stable

---

## Phase 2 — QUS integration

### Goal

Consume structured query intent.

### Tasks

1. Add QUS client interface
2. Implement HTTP client adapter
3. Add timeout handling
4. Add fallback mode when QUS fails
5. Add mocked client tests

### Done criteria

* raw search request becomes QUS-enriched internal request

---

## Phase 3 — Search plan builder

### Goal

Translate intent into execution stages.

### Tasks

1. Merge explicit filters with inferred QUS filters
2. Apply default filters
3. Build stage list
4. Resolve applied sort
5. Build facet request config
6. Add unit tests

### Done criteria

* service produces deterministic `SearchPlan`

---

## Phase 4 — OpenSearch query builder

### Goal

Generate DSL for stage 1 and stage 2.

### Tasks

1. Add OpenSearch client interface
2. Implement bool/filter/multi_match builder
3. Add field boosts config
4. Add stage-specific query modes
5. Add search_after support
6. Add unit tests with golden DSL snapshots

### Done criteria

* exact and fallback DSL generated correctly

---

## Phase 5 — Stage executor

### Goal

Run stage-based search with fallback.

### Tasks

1. Execute stage 1
2. Inspect total hits
3. Execute stage 2 if below threshold
4. Record applied stage
5. Add tests for stage switching behavior

### Done criteria

* fallback runs only when needed
* stage metadata preserved in response

---

## Phase 6 — Facets

### Goal

Return correct filtered aggregations.

### Tasks

1. Build facet aggregations from config
2. Support self-filter exclusion
3. Map aggregation results to API response
4. Add unit tests and integration fixtures

### Done criteria

* facet counts consistent with filter rules

---

## Phase 7 — Result mapping and pagination

### Goal

Produce client-ready output.

### Tasks

1. Map hits to search items
2. Generate cursor from `search_after`
3. Add `hasNextPage`
4. Add stable tiebreakers
5. Add tests for pagination token integrity

### Done criteria

* deep pagination works with stable ordering

---

## Phase 8 — Observability and hardening

### Goal

Make MVP operable.

### Tasks

1. Add Prometheus metrics
2. Add per-stage timing
3. Add request correlation ID
4. Add OpenSearch timeout handling
5. Add config validation
6. Add dependency health checks

### Done criteria

* service is observable in staging/production

---

## Phase 9 — Golden behavior suite

### Goal

Lock search behavior.

### Tasks

1. Add golden tests for:

   * exact match
   * fallback triggered
   * explicit filters
   * inferred QUS filters
   * sort modes
   * facet self-exclusion
   * cursor pagination
2. Add fixture-based OpenSearch response mapping tests

### Done criteria

* orchestration behavior is regression-safe

---

# 17. Prompt Pack for Claude Code

Use these sequentially.

## Prompt 1 — Repository bootstrap

```text
Create a Go service named search-orchestrator that exposes:
- GET /healthz
- POST /v1/search

Requirements:
- use idiomatic Go project structure
- use net/http or chi
- add config loading from yaml
- define request/response structs for SearchRequest and SearchResponse
- add structured logging
- return a deterministic stubbed response for /v1/search
- include Makefile targets: run, test, fmt
- include unit tests for config loading and handler smoke test
```

## Prompt 2 — Request validation and API contract

```text
Implement request validation and stable response models for the Go search-orchestrator service.

Requirements:
- validate locale, market, page size, and sort enum
- define pageInfo, items, facets, and debug response models
- reject invalid requests with structured errors
- add table-driven unit tests
Keep the API contract simple and deterministic.
```

## Prompt 3 — QUS integration

```text
Implement QUS integration for the Go search-orchestrator service.

Requirements:
- create a QUS client interface so it can be mocked in tests
- call QUS before building the search plan
- support timeout handling
- if QUS fails, degrade gracefully by using the raw query and explicit request filters only
- add unit tests and mocked client tests
Do not tightly couple the service to a specific transport.
```

## Prompt 4 — Search planning

```text
Implement a SearchPlan builder for the Go search-orchestrator service.

Requirements:
- merge explicit request filters with inferred QUS filters
- apply configured default filters
- resolve applied sort from allowed values
- build a two-stage execution plan: exact then fallback_partial
- attach facet requests from config
- add table-driven unit tests
Keep the implementation explicit and readable.
```

## Prompt 5 — OpenSearch query builder

```text
Implement an OpenSearch query builder for the Go search-orchestrator service.

Requirements:
- create an OpenSearch client interface for mocking
- generate DSL for two stages: exact and fallback_partial
- support bool queries, filter clauses, multi_match field boosts, sort clauses, and search_after
- load field boosts and stage config from yaml
- add golden tests for generated DSL
Do not recreate a Java-style abstraction hierarchy.
```

## Prompt 6 — Stage execution and fallback

```text
Implement staged search execution for the Go search-orchestrator service.

Requirements:
- execute stage 1 against OpenSearch
- inspect hit count
- if below threshold, execute stage 2
- record which stage was applied
- preserve stage timing and debug metadata
- add tests for fallback behavior using mocked OpenSearch responses
Keep stage execution deterministic and explicit.
```

## Prompt 7 — Facets

```text
Implement faceting for the Go search-orchestrator service.

Requirements:
- support terms and range facets from config
- support self-filter exclusion per facet
- map OpenSearch aggregations into the API response model
- add unit tests and golden tests for facet aggregation requests and response mapping
Keep the facet logic easy to reason about.
```

## Prompt 8 — Result mapping and pagination

```text
Implement result mapping and cursor pagination for the Go search-orchestrator service.

Requirements:
- map OpenSearch hits into SearchItem response objects
- generate nextCursor using search_after values encoded as base64 JSON
- compute hasNextPage safely
- use stable tie-breaker sorting
- add unit tests for pagination token generation and parsing
Keep the implementation production-readable.
```

## Prompt 9 — Observability and hardening

```text
Add production MVP hardening to the Go search-orchestrator service.

Requirements:
- per-stage timing
- Prometheus metrics
- request correlation ID
- timeout handling for QUS and OpenSearch calls
- config validation
- health endpoint should verify config is loaded and dependencies can be initialized
- add tests where practical
Keep implementation lightweight and idiomatic.
```

---

# 18. Engineering Constraints for Claude Code

## Coding rules

* use idiomatic Go
* constructor-based dependency injection
* interfaces only where they serve testing or integration boundaries
* no reflection-heavy frameworks
* no global mutable state except startup config
* context propagation everywhere
* table-driven tests
* golden tests for query DSL and orchestration behavior
* explicit errors
* stable JSON schemas

## Non-goals

* do not port the Java workflow engine literally
* do not port Spring patterns
* do not introduce generic step frameworks without current need
* do not add merch-rule hooks in MVP
* do not add premature concurrency or worker pools

---

# 19. Acceptance Criteria

The MVP is complete when the following are true:

1. A request like:

```json
{
  "query": "cheap chiken burger under 20",
  "locale": "en-GB",
  "market": "uk",
  "filters": {
    "availability": ["in_stock"]
  },
  "page": {
    "size": 24
  },
  "sort": "relevance"
}
```

produces:

* QUS-enriched search planning
* exact stage execution
* fallback execution only if needed
* filtered results
* facet results
* stable pagination cursor

2. If QUS fails:

* raw query still executes
* explicit filters still apply
* response remains usable

3. If OpenSearch stage 1 returns too few hits:

* stage 2 executes automatically
* applied stage is reported correctly

4. Behavior is covered by:

* unit tests
* golden DSL tests
* mocked QUS/OpenSearch integration tests

---

# 20. Recommended MVP Milestones

| Milestone | Outcome                    | Duration |
| --------- | -------------------------- | -------- |
| M1        | API skeleton + validation  | 2–3 days |
| M2        | QUS integration + planning | 3–5 days |
| M3        | DSL builder + stage 1      | 4–6 days |
| M4        | fallback + facets          | 4–6 days |
| M5        | pagination + mapping       | 3–4 days |
| M6        | tests + hardening          | 3–5 days |

Total MVP: **3–5 weeks** for one strong engineer.

---

# 21. Final Instruction Set for Claude Code

```text
Build an MVP Search Orchestrator in Go.

The service must:
- expose POST /v1/search
- validate incoming search requests
- call QUS to enrich raw queries into structured intent
- merge explicit filters, inferred QUS filters, and configured default filters
- build a deterministic two-stage search plan: exact then fallback_partial
- generate OpenSearch DSL natively in Go
- execute stage 1 and stage 2 only when needed
- return mapped hits, facets, applied stage, and cursor pagination
- support terms and range facets with self-filter exclusion
- degrade safely if QUS is unavailable
- include unit tests, golden DSL tests, mocked integration tests, Prometheus metrics, structured logging, and timeouts

Constraints:
- do not port the Java workflow engine literally
- do not add merchandising, redirects, or personalization in MVP
- keep the design idiomatic Go and production-readable
- prioritize deterministic behavior, explicit stages, and clear package boundaries
```

# 22. Delivery Order

1. bootstrap
2. validation
3. QUS integration
4. search planning
5. OpenSearch DSL builder
6. stage execution
7. facets
8. pagination and mapping
9. observability
10. golden regression suite
