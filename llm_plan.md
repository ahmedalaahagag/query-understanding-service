# Plan: Add LLM support QUS with an LLM-Assisted Semantic Interpretation Stage

## Objective

Change QUS from a fully deterministic pipeline into a **hybrid architecture**:

* **deterministic preprocessing**
* **LLM semantic interpretation**
* **deterministic validation and enforcement**
* **final structured AnalyzeResponse**

The goal is to remove unnecessary pipeline complexity while preserving:

* deterministic outputs
* business-rule enforcement
* OpenSearch compatibility
* safe fallback behavior

The LLM should become a **semantic parser**, not the final authority.

---

# 1. Target Architecture

## New QUS flow

```text
Raw Query
   │
   ▼
Deterministic Preprocessing
   - normalize
   - tokenize
   - cheap cleanup
   - market/locale context
   │
   ▼
LLM Semantic Interpretation
   - canonical rewrite
   - filters
   - sort
   - candidate concepts
   - confidence
   │
   ▼
Deterministic Validation + Enrichment
   - validate schema
   - validate filters/operators
   - validate concept IDs or resolve labels
   - enforce market/locale constraints
   - reject unsupported outputs
   │
   ▼
Final AnalyzeResponse
```

---

# 2. Scope of the Change

## Remove from primary online path

These should no longer be core pipeline stages in MVP if the LLM handles them:

* rule-heavy comprehension stage
* complex ambiguity resolution logic
* hand-built semantic rewrite logic
* large parts of concept ranking heuristics
* custom intent extraction pipeline stages

## Keep deterministic

These remain in code:

* normalization
* tokenization
* request validation
* supported filter/operator enforcement
* concept lookup / validation against OpenSearch or config
* fallback behavior
* response assembly
* observability
* low-level guards

## Optional to keep as fallback or secondary signal

* synonym expansion
* compound handling
* spell correction via OpenSearch
* concept shingling

These can remain as:

* fallback path when LLM fails
* enrichment path before/after LLM
* offline evaluation reference

---

# 3. Product Goal

## Input

```json
{
  "query": "cheap chiken burger under 20",
  "locale": "en-GB",
  "market": "uk"
}
```

## LLM intermediate output

```json
{
  "normalizedQuery": "cheap chicken burger under 20",
  "rewrites": ["cheap chicken burger"],
  "candidateConcepts": [
    {
      "label": "burger",
      "field": "product_type",
      "confidence": 0.94
    }
  ],
  "filters": [
    {
      "field": "price",
      "operator": "lt",
      "value": 20,
      "confidence": 0.96
    }
  ],
  "sort": {
    "field": "price",
    "direction": "asc",
    "confidence": 0.72
  },
  "confidence": 0.89,
  "warnings": []
}
```

## Final QUS output after validation

```json
{
  "originalQuery": "cheap chiken burger under 20",
  "normalizedQuery": "cheap chicken burger under 20",
  "tokens": [
    {"value": "cheap", "normalized": "cheap", "position": 0},
    {"value": "chiken", "normalized": "chiken", "position": 1},
    {"value": "burger", "normalized": "burger", "position": 2},
    {"value": "under", "normalized": "under", "position": 3},
    {"value": "20", "normalized": "20", "position": 4}
  ],
  "rewrites": ["cheap chicken burger"],
  "concepts": [
    {
      "id": "product_type_burger",
      "label": "burger",
      "matchedText": "burger",
      "field": "product_type",
      "score": 0.94,
      "source": "llm",
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
  },
  "warnings": []
}
```

---

# 4. Design Principles

1. **LLM is advisory, not authoritative**
2. **All outputs must be schema-validated**
3. **All business semantics must be code-validated**
4. **Every inferred concept/filter must be resolvable**
5. **Fallback must exist for model failure**
6. **Behavior must be regression-tested**
7. **Prompting must stay narrow and deterministic**

---

# 5. Repository Layout

Use the existing QUS service structure and introduce LLM-specific packages.

```text
qus/
├── cmd/qus/
│   └── main.go
├── internal/
│   ├── api/
│   ├── app/
│   ├── config/
│   ├── model/
│   ├── pipeline/
│   ├── preprocess/
│   ├── llm/
│   ├── validation/
│   ├── concept/
│   ├── filters/
│   ├── fallback/
│   ├── opensearch/
│   ├── metrics/
│   └── testutil/
├── configs/
│   ├── qus.yaml
│   ├── allowed_filters.yaml
│   ├── allowed_sorts.yaml
│   ├── llm_prompt.txt
│   └── llm_examples.json
├── testdata/
│   ├── golden/
│   └── fixtures/
└── README.md
```

---

# 6. New Logical Pipeline

## Step 1 — Preprocess

Responsibilities:

* lowercase
* whitespace cleanup
* tokenize
* locale/market attachment
* raw token capture

Output:

* `PreprocessedQuery`

## Step 2 — LLM Semantic Parse

Responsibilities:

* produce canonical rewrite
* infer filters
* infer sort
* propose concepts
* produce confidence and warnings

Output:

* `LLMParseResult`

## Step 3 — Validate and Normalize LLM Output

Responsibilities:

* validate JSON schema
* reject unsupported filters
* reject unknown operators
* normalize field names to internal canonical names
* strip invalid sort directives

Output:

* `ValidatedSemanticIntent`

## Step 4 — Resolve Concepts

Responsibilities:

* map proposed labels/aliases to real concept IDs
* query concept index or config source
* keep only resolvable concepts
* attach deterministic score/source metadata

Output:

* `ResolvedConcepts`

## Step 5 — Finalize Response

Responsibilities:

* merge deterministic tokens + validated semantics
* attach warnings
* set fallback/debug flags if needed

Output:

* `AnalyzeResponse`

---

# 7. LLM Contract

## Request to model

The model should receive:

* user query
* locale
* market
* allowed filter definitions
* allowed sort definitions
* allowed concept fields
* a strict instruction to output JSON only

## Response schema

```go
type LLMParseResult struct {
    NormalizedQuery   string                `json:"normalizedQuery"`
    Rewrites          []string              `json:"rewrites"`
    CandidateConcepts []LLMCandidateConcept `json:"candidateConcepts"`
    Filters           []LLMFilter           `json:"filters"`
    Sort              *LLMSort              `json:"sort,omitempty"`
    Confidence        float64               `json:"confidence"`
    Warnings          []string              `json:"warnings,omitempty"`
}
```

### Candidate concept

```go
type LLMCandidateConcept struct {
    Label      string   `json:"label"`
    Field      string   `json:"field"`
    Confidence float64  `json:"confidence"`
    Aliases    []string `json:"aliases,omitempty"`
}
```

### Filter

```go
type LLMFilter struct {
    Field      string      `json:"field"`
    Operator   string      `json:"operator"`
    Value      interface{} `json:"value"`
    Confidence float64     `json:"confidence"`
}
```

### Sort

```go
type LLMSort struct {
    Field      string  `json:"field"`
    Direction  string  `json:"direction"`
    Confidence float64 `json:"confidence"`
}
```

---

# 8. Validation Layer

This is the most important part of the change.

## Filter validation

Validate against config:

* allowed fields
* allowed operators
* expected value type
* market restrictions

Example:

```yaml
filters:
  - field: price
    operators: [lt, lte, gt, gte, eq]
    value_type: number

  - field: dietary
    operators: [in, eq]
    value_type: string_list
```

## Sort validation

Validate against a small allowlist:

```yaml
sorts:
  - relevance
  - price_asc
  - price_desc
  - newest
```

The LLM should not invent arbitrary sort fields.

## Concept validation

Never trust concept labels directly.

Validation path:

1. LLM proposes concept label + field
2. service resolves against concept index/config
3. only matched known concepts survive
4. unresolved concepts are dropped and warning emitted

---

# 9. Fallback Strategy

## Fallback levels

### Level 1 — LLM succeeds and validation succeeds

Use full hybrid result

### Level 2 — LLM succeeds but partial validation fails

Use:

* normalized query if acceptable
* valid filters only
* valid concepts only
* drop invalid parts

### Level 3 — LLM times out or returns invalid output

Fall back to deterministic minimal path:

* normalized raw query
* tokens
* explicit deterministic filters only if any local rules exist
* no inferred concepts
* no inferred sort

### Level 4 — catastrophic dependency failure

Return safe response with:

* original query
* tokens
* warnings
* no semantic enrichments

---

# 10. Configuration Changes

## Main QUS config

```yaml
llm:
  enabled: true
  timeout_ms: 800
  max_retries: 1
  min_confidence: 0.65
  fail_open: true
  prompt_file: configs/llm_prompt.txt

validation:
  drop_invalid_filters: true
  drop_invalid_sorts: true
  drop_unresolved_concepts: true

fallback:
  use_deterministic_on_llm_failure: true
```

## Allowed filter config

```yaml
filters:
  - field: price
    operators: [lt, lte, gt, gte, eq]
    type: number

  - field: dietary
    operators: [eq, in]
    type: string_list

  - field: availability
    operators: [eq, in]
    type: string_list
```

## Allowed sort config

```yaml
sorts:
  - key: relevance
  - key: price_asc
  - key: price_desc
  - key: newest
```

---

# 11. Prompt Design

## Prompt content must include

* strict role: semantic query parser
* business context: ecommerce/product search
* explicit allowed filters and sorts
* instructions to avoid inventing unsupported fields
* instructions to return empty arrays instead of guessing
* JSON-only output
* a few examples

## Prompt rules

* no chain-of-thought output
* no prose
* no explanations unless inside `warnings`
* prefer abstention over guessing

---

# 12. API Changes

The public QUS API can remain the same.

## No required external contract break

Keep:

```http
POST /v1/analyze
```

## Optional debug endpoint

Add:

```http
POST /v1/analyze/debug
```

Debug payload may include:

* preprocessed tokens
* raw LLM output
* validation rejections
* fallback flags

Keep behind config flag only.

---

# 13. Step-by-Step Delivery Plan for Claude Code

## Phase 0 — Introduce LLM boundary

### Goal

Create pluggable LLM client and response schema.

### Tasks

1. Add `llm` package
2. Define `LLMClient` interface
3. Define `LLMParseResult` structs
4. Add config for LLM enablement, timeout, confidence
5. Add mocked implementation for tests

### Done criteria

* service compiles with LLM boundary in place
* no behavior change yet

---

## Phase 1 — Preprocess and output preservation

### Goal

Preserve current deterministic preprocessing as the stable front of the pipeline.

### Tasks

1. Extract normalization/tokenization into `preprocess` package
2. Ensure tokens are always produced independent of LLM
3. Add unit tests
4. Add golden tests for preprocessing

### Done criteria

* preprocessing stable and independent

---

## Phase 2 — LLM semantic parser integration

### Goal

Add model call that returns structured intent.

### Tasks

1. Implement prompt builder
2. Implement LLM call wrapper
3. Parse structured JSON result
4. Add timeout and retry behavior
5. Add mocked tests for:

    * valid output
    * invalid JSON
    * timeout
    * missing fields

### Done criteria

* QUS can call the model and parse a typed result

---

## Phase 3 — Validation layer

### Goal

Make model outputs safe.

### Tasks

1. Implement filter validation
2. Implement sort validation
3. Implement confidence threshold handling
4. Drop invalid fields/operators
5. Add warnings for rejected elements
6. Add unit tests

### Done criteria

* invalid model outputs cannot leak into final response

---

## Phase 4 — Concept resolution

### Goal

Turn model-proposed concepts into valid internal concepts.

### Tasks

1. Implement concept resolver interface
2. Resolve candidate label/field pairs against concept index
3. Keep best valid match only
4. Map into final `ConceptMatch`
5. Add tests for:

    * exact label match
    * alias match
    * unresolved concept
    * multiple candidates

### Done criteria

* concepts in final output are always valid internal concepts

---

## Phase 5 — Final response assembly

### Goal

Build final `AnalyzeResponse` from deterministic + validated semantic inputs.

### Tasks

1. Merge preprocessing output with validated LLM result
2. Populate rewrites
3. Populate filters
4. Populate sort
5. Populate concepts
6. Populate warnings/debug flags

### Done criteria

* public API returns stable enriched response

---

## Phase 6 — Fallback path

### Goal

Make the system operationally safe.

### Tasks

1. Add deterministic fallback mode when LLM fails
2. Add fallback metadata in debug
3. Add unit tests for all fallback levels
4. Verify service remains usable without model output

### Done criteria

* online serving is resilient to LLM failure

---

## Phase 7 — Observability and evaluation hooks

### Goal

Make the new hybrid path measurable.

### Tasks

1. Add metrics:

    * llm_request_count
    * llm_latency_ms
    * llm_failure_count
    * validation_rejection_count
    * fallback_count
2. Add correlation IDs
3. Add sampling for debug logs
4. Add offline evaluation output capture if needed

### Done criteria

* model-driven behavior can be monitored

---

## Phase 8 — Golden behavior suite

### Goal

Lock expected behavior.

### Tasks

1. Add golden tests for:

    * price extraction
    * sort extraction
    * concept inference
    * typo rewrite
    * ambiguity cases
    * invalid model outputs
    * fallback mode
2. Add fixtures for mocked LLM responses

### Done criteria

* regressions are visible immediately

---

# 14. Prompt Pack for Claude Code

## Prompt 1 — LLM boundary

```text
Refactor the Go QUS service to support an LLM-assisted semantic interpretation stage.

Requirements:
- add an llm package
- define an LLMClient interface
- define typed structs for LLMParseResult, candidate concepts, filters, and sort
- add config fields for llm enabled flag, timeout, min confidence, and fail-open behavior
- add mocked implementations for tests
Do not change the external /v1/analyze contract yet.
```

## Prompt 2 — Preprocess extraction

```text
Refactor the Go QUS service so normalization and tokenization are a standalone deterministic preprocess stage.

Requirements:
- extract preprocess logic into a dedicated package
- ensure tokens are always produced whether the LLM succeeds or fails
- add unit tests and golden tests for preprocessing
Keep behavior deterministic and simple.
```

## Prompt 3 — LLM integration

```text
Implement the LLM semantic interpretation stage for the Go QUS service.

Requirements:
- build a prompt from query, locale, market, allowed filters, and allowed sorts
- call the LLM through an interface
- parse a typed LLMParseResult response
- handle timeout, retry, invalid JSON, and empty output
- add mocked tests for success and failure scenarios
Do not trust the model output yet; validation comes next.
```

## Prompt 4 — Validation layer

```text
Implement a validation layer for the LLM semantic interpretation output.

Requirements:
- validate filters against configured allowed fields/operators/types
- validate sort against an allowlist
- enforce confidence thresholds
- reject or drop invalid model-generated outputs
- attach warnings describing rejections
- add table-driven unit tests
Keep the implementation explicit and production-readable.
```

## Prompt 5 — Concept resolution

```text
Implement deterministic concept resolution for LLM-proposed candidate concepts in the Go QUS service.

Requirements:
- resolve concept labels and fields against a concept repository interface
- support exact label and alias resolution
- drop unresolved concepts
- map resolved concepts into the existing AnalyzeResponse concept model
- add unit tests for exact, alias, ambiguous, and unresolved cases
Do not trust model-provided concept IDs directly.
```

## Prompt 6 — Final response assembly

```text
Integrate the LLM semantic stage into the Go QUS /v1/analyze pipeline.

Requirements:
- merge preprocessing output with validated semantic intent
- populate normalizedQuery, rewrites, filters, sort, and concepts
- preserve deterministic tokens
- include warnings when model output is partially rejected
- keep the external AnalyzeResponse contract stable
Add unit tests and golden tests.
```

## Prompt 7 — Fallback behavior

```text
Add operational fallback behavior to the Go QUS service.

Requirements:
- if the LLM times out or returns invalid output, fall back to deterministic preprocessing-only behavior
- ensure /v1/analyze still returns a usable response
- add debug metadata or warnings indicating fallback occurred
- add unit tests for all fallback paths
Keep fail-open behavior configurable.
```

## Prompt 8 — Observability and hardening

```text
Add production hardening for the LLM-assisted QUS service.

Requirements:
- Prometheus metrics for llm requests, failures, latency, validation rejections, and fallbacks
- request correlation ID
- timeout handling
- config validation
- optional debug endpoint to inspect preprocess result, raw semantic parse, validation rejections, and final response
Add tests where practical.
```

---

# 15. Engineering Constraints for Claude Code

## Coding rules

* use idiomatic Go
* constructor-based dependency injection
* context propagation everywhere
* explicit interfaces only at integration boundaries
* no reflection-heavy framework usage
* no global mutable runtime state
* table-driven tests
* golden tests for behavioral contract
* explicit warnings for dropped model output

## Non-goals

* do not make the LLM the sole authority
* do not accept arbitrary model-generated filters/sorts/concepts
* do not rebuild the old graph pipeline under a different name
* do not add agent loops or tool-chaining for MVP
* do not add self-learning behavior

---

# 16. Acceptance Criteria

The change is complete when:

1. `/v1/analyze` still returns the existing response shape
2. The model can produce:

    * rewritten query
    * inferred filters
    * inferred sort
    * candidate concepts
3. Invalid model outputs are rejected safely
4. Unresolved concepts never appear in final output
5. If the model fails, the service still returns a usable deterministic response
6. Metrics show:

    * model usage
    * latency
    * failures
    * fallback rate
    * rejection rate
7. Behavior is covered by:

    * unit tests
    * mocked LLM tests
    * golden tests

---

# 17. Recommended MVP Milestones

| Milestone | Outcome                         | Duration |
| --------- | ------------------------------- | -------- |
| M1        | LLM interface + config          | 1–2 days |
| M2        | preprocess extraction           | 1–2 days |
| M3        | semantic parse integration      | 3–5 days |
| M4        | validation + concept resolution | 4–6 days |
| M5        | fallback + response assembly    | 2–4 days |
| M6        | tests + observability           | 3–5 days |

Total change: **2–4 weeks** for one strong engineer.

---

# 18. Final Instruction Set for Claude Code

```text
Refactor the Go Query Understanding Service into a hybrid architecture where an LLM performs semantic interpretation but deterministic Go code remains the final authority.

The service must:
- keep deterministic preprocessing for normalization and tokenization
- call an LLM to infer rewritten query, filters, sort, and candidate concepts
- validate all model outputs against configured allowed fields, operators, and sorts
- resolve model-proposed concepts against a deterministic concept repository
- drop invalid or unresolved outputs safely
- preserve the existing /v1/analyze response contract
- fail open with deterministic fallback behavior if the LLM is unavailable or invalid
- include unit tests, golden tests, mocked LLM integration tests, Prometheus metrics, structured logging, and timeouts

Constraints:
- do not make the LLM the sole authority
- do not trust arbitrary model-generated fields
- do not port the old Java graph pipeline
- keep the design idiomatic Go and production-readable
- prioritize deterministic behavior, validation, and safe degradation
```

---

# 19. Delivery Order

1. add LLM boundary
2. extract deterministic preprocessing
3. integrate semantic parse call
4. validate model outputs
5. resolve concepts deterministically
6. assemble final response
7. add fallback behavior
8. add observability
9. lock behavior with golden tests
