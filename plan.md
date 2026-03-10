# MVP Plan for Rebuilding QUS in Go

## Objective

Build an **MVP Query Understanding Service (QUS)** in Go that delivers the practical value of the existing Java QUS without reproducing all of its graph complexity.

The MVP should support:

* query normalization
* tokenization
* typo tolerance via OpenSearch-native mechanisms
* optional synonym / alias expansion
* basic compound-word handling
* concept recognition against a controlled concept index
* simple ambiguity resolution
* basic comprehension rules
* deterministic output for downstream search orchestration

The MVP should **not** attempt full semantic graph parity with Java.

---

# 1. Product Goal

The MVP QUS exists to transform a raw user query into structured search intent.

### Input

```json
{
  "query": "cheap chiken burger",
  "locale": "en-GB",
  "market": "uk",
  "processorNames": []
}
```

### Output

```json
{
  "originalQuery": "cheap chiken burger",
  "normalizedQuery": "cheap chicken burger",
  "tokens": [
    {"value": "cheap", "normalized": "cheap", "position": 0},
    {"value": "chiken", "normalized": "chicken", "position": 1},
    {"value": "burger", "normalized": "burger", "position": 2}
  ],
  "rewrites": [
    "cheap chicken burger"
  ],
  "concepts": [
    {
      "id": "product_type_burger",
      "label": "burger",
      "matchedText": "burger",
      "score": 0.98,
      "source": "exact"
    }
  ],
  "filters": [
    {
      "field": "price",
      "operator": "lt",
      "value": 20
    }
  ],
  "sort": null,
  "warnings": []
}
```

This output is the contract for downstream search.

---

# 2. MVP Scope

## In scope

### Phase 1 core features

* HTTP API
* config loading
* tokenization
* normalization
* OpenSearch-backed spell suggestion lookup
* single-path corrected rewrite generation
* synonym expansion from config
* simple compound-word split/join rules
* concept lookup from OpenSearch index
* basic overlap resolution
* comprehension rules for price / sort / basic filters
* structured JSON response
* metrics / logging
* unit and golden tests

## Out of scope

* full graph-based query representation
* multi-path query graph branching
* hypernym networks
* weighted multigraph traversal
* runtime pipeline mutation
* admin UI
* linguistic CRUD APIs
* advanced ML / embeddings
* exact behavioral parity with Java
* automatic learning / feedback loops

---

# 3. Architecture

## Target service shape

```text
Client
  │
  ▼
QUS API (Go)
  │
  ├── Normalizer
  ├── Tokenizer
  ├── Spell Resolver
  ├── Synonym Expander
  ├── Compound Handler
  ├── Concept Recognizer
  ├── Ambiguity Resolver
  ├── Comprehension Engine
  │
  ▼
Structured Query Intent JSON
```

## External dependencies

* OpenSearch
* config files in YAML or JSON
* optional Redis only if needed later, not for MVP

## OpenSearch Native Capabilities (Build Approach)

OpenSearch 2.10+ natively provides the foundation for almost all QUS features. We lean on these instead of building custom logic.

| QUS Feature | OpenSearch Native Alternative | QUS Approach |
|---|---|---|
| Spell correction | `fuzzy` queries, `did_you_mean` suggester | Use native suggesters |
| Synonyms | Synonym token filters in analyzer config | **Application-level** — need weighted boosting per type (SYN=0.9x, HYP=0.8x) |
| Compound words | `word_delimiter` filter, `shingle` filter | Application-level split/join + shingle generation |
| Multi-field boosting | `multi_match` with `cross_fields` + per-field boosts | Use native multi_match |
| Partial matching | `minimum_should_match: 75%` on `multi_match` | Use native |
| KNN vector search | Native `knn` query | Future |
| Hybrid search | OpenSearch 2.10+ native hybrid search with score normalization (RRF) | Future |

### Phase-level implications

* **Phase 2 (Spell)**: Use `did_you_mean` suggester / `fuzzy` queries — no custom edit-distance logic
* **Phase 3 (Synonyms/Hypernyms)**: Application-level expansion (not OpenSearch synonym filters) — weighted boosting per type is critical for ranking quality. Query existing MXP linguistic dictionary index.
* **Phase 3 (Compounds)**: Application-level split/join with shingle generation
* **Phase 4 (Concepts)**: `multi_match` with `cross_fields` + per-field boosts for concept recognition
* **Future**: Native KNN + hybrid search (RRF) for semantic matching

---

# 4. Design Principles

1. **Flatten complexity early**

   * do not rebuild the Java graph model for MVP

2. **One canonical rewrite**

   * generate one best normalized query first
   * optionally keep alternates later

3. **Deterministic output**

   * same input and same config should always return same result

4. **Config-driven where cheap**

   * synonyms, compounds, comprehension rules

5. **OpenSearch does the heavy lifting**

   * spell support
   * concept index lookup
   * ranking support

6. **Golden tests define behavior**

   * behavior contract matters more than internal implementation elegance

---

# 5. Recommended Repository Layout

```text
qus/
├── cmd/qus/
│   └── main.go
├── internal/
│   ├── api/
│   ├── app/
│   ├── config/
│   ├── pipeline/
│   ├── model/
│   ├── normalizer/
│   ├── tokenizer/
│   ├── spell/
│   ├── synonym/
│   ├── compound/
│   ├── concept/
│   ├── ambiguity/
│   ├── comprehension/
│   ├── opensearch/
│   ├── metrics/
│   └── testutil/
├── configs/
│   ├── qus.yaml
│   ├── synonyms.en.yaml
│   ├── compounds.en.yaml
│   └── comprehension.en.yaml
├── testdata/
│   ├── golden/
│   └── fixtures/
├── Makefile
├── go.mod
└── README.md
```

---

# 6. Domain Model

## Main request model

```go
type AnalyzeRequest struct {
    Query          string   `json:"query"`
    Locale         string   `json:"locale"`
    Market         string   `json:"market"`
    ProcessorNames []string `json:"processorNames,omitempty"`
}
```

## Main response model

```go
type AnalyzeResponse struct {
    OriginalQuery   string         `json:"originalQuery"`
    NormalizedQuery string         `json:"normalizedQuery"`
    Tokens          []Token        `json:"tokens"`
    Rewrites        []string       `json:"rewrites"`
    Concepts        []ConceptMatch `json:"concepts"`
    Filters         []Filter       `json:"filters"`
    Sort            *SortSpec      `json:"sort,omitempty"`
    Warnings        []string       `json:"warnings,omitempty"`
}
```

## Token model

```go
type Token struct {
    Value      string `json:"value"`
    Normalized string `json:"normalized"`
    Position   int    `json:"position"`
}
```

## Concept model

```go
type ConceptMatch struct {
    ID          string   `json:"id"`
    Label       string   `json:"label"`
    MatchedText string   `json:"matchedText"`
    Field       string   `json:"field,omitempty"`
    Score       float64  `json:"score"`
    Source      string   `json:"source"` // exact|synonym|spell|compound
    Start       int      `json:"start"`
    End         int      `json:"end"`
}
```

## Filter model

```go
type Filter struct {
    Field    string      `json:"field"`
    Operator string      `json:"operator"`
    Value    interface{} `json:"value"`
}
```

---

# 7. Pipeline Definition

## Ordered pipeline for MVP

### Step 1 — Normalize

Tasks:

* lowercase
* trim whitespace
* collapse repeated spaces
* strip noise punctuation except meaningful separators
* normalize unicode where useful

Output:

* normalized raw string

### Step 2 — Tokenize

Tasks:

* split on whitespace
* preserve positions
* optionally preserve original offsets

Output:

* ordered tokens

### Step 3 — Spell resolve

Tasks:

* query OpenSearch suggester or rely on controlled dictionary endpoint
* correct only obviously misspelled tokens
* keep confidence threshold
* avoid aggressive changes on short tokens, brand-like terms, SKUs

Output:

* corrected token list
* warning if low confidence

### Step 4 — Synonym expansion

Tasks:

* apply static config synonyms
* produce canonical token replacements
* do not explode the query into many branches for MVP

Example:

* `veggie` → `vegetarian`
* `coke` → `coca cola`

Output:

* canonical rewrite

### Step 5 — Compound handling

Tasks:

* split configured compounds
* join configured multi-token compounds
* run only from config or simple heuristics

Examples:

* `crewneck` → `crew neck`
* `ice cream` → `icecream` if domain requires

Output:

* updated token sequence

### Step 6 — Concept recognition

Tasks:

* build token shingles 1..N
* query concept index in OpenSearch
* score matches using simple scheme:

  * exact = 1.0
  * synonym = 0.9
  * spell = 0.7
  * compound = 0.8
* keep top valid matches

Output:

* concept candidates

### Step 7 — Ambiguity resolution

Tasks:

* remove overlapping weaker matches
* prefer longest span
* prefer highest score
* prefer more specific field if tie

Example:

* `chicken burger` should beat separate weak `chicken` and `burger` concepts if domain logic says product type phrase is better

Output:

* resolved concepts

### Step 8 — Comprehension

Tasks:

* detect price phrases:

  * under 20
  * less than 10
  * cheaper than 5
* detect sort:

  * cheapest
  * lowest price
  * newest
* detect simple filters:

  * vegan
  * spicy
  * gluten free

Output:

* structured filters and sort directives

### Step 9 — Response build

Tasks:

* emit canonical normalized query
* emit one rewrite
* include resolved concepts
* include filters/sort
* include warnings

---

# 8. OpenSearch Dependencies

## 8.1 Spell support

For MVP use OpenSearch native capabilities:

* `did_you_mean` suggester (preferred) or `term`/`phrase` suggester
* `fuzzy` queries as alternative for inline correction
* fallback to no correction if confidence is weak

Do not build custom Damerau-Levenshtein or edit-distance logic.

## 8.2 Concept index

Create a dedicated concept index, for example:

```json
{
  "id": "concept_burger",
  "label": "burger",
  "field": "product_type",
  "aliases": ["burgers"],
  "locale": "en-GB",
  "market": "uk",
  "weight": 10
}
```

Recommended fields:

* `id`
* `label`
* `aliases`
* `field`
* `locale`
* `market`
* `weight`
* `status`

## 8.3 Search strategy for concepts

For each shingle:

* exact label match
* alias match
* maybe fuzzy match for long shingles only

Do not overdo fuzzy concept lookup. It creates garbage.

---

# 9. Config Files

## Main config

```yaml
server:
  port: 8080

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
  min_token_length: 4
  confidence_threshold: 0.85

concept:
  shingle_max_size: 4
  max_matches_per_span: 3

ambiguity:
  prefer_longest_span: true
  min_score_delta: 0.05
```

## Synonym config

```yaml
locale: en-GB
entries:
  - canonical: vegetarian
    variants: ["veggie"]
  - canonical: coca cola
    variants: ["coke"]
```

## Compound config

```yaml
locale: en-GB
split:
  - crewneck
  - lunchbox
join:
  - source: ["ice", "cream"]
    target: "icecream"
```

## Comprehension rules

```yaml
price_rules:
  - pattern: '(under|less than|cheaper than)\s+(\d+)'
    field: price
    operator: lt

sort_rules:
  - pattern: '(cheapest|lowest price)'
    field: price
    direction: asc
```

---

# 10. API Endpoints

## Health

```http
GET /healthz
```

## Analyze

```http
POST /v1/analyze
Content-Type: application/json
```

## Optional debug endpoint

```http
POST /v1/analyze/debug
```

Debug output may include step-by-step pipeline state. Useful for parity work. Keep behind config flag.

---

# 11. Step-by-Step Delivery Plan for Claude Code

## Phase 0 — Bootstrap

### Goal

Create working Go service skeleton.

### Tasks

1. Initialize Go module
2. Add HTTP server
3. Add config loader
4. Add health endpoint
5. Add `/v1/analyze` endpoint
6. Define request/response models
7. Add structured logging

### Done criteria

* service runs locally
* returns stubbed response

---

## Phase 1 — Core normalization and tokenization

### Goal

Produce deterministic tokenized query output.

### Tasks

1. Implement normalizer
2. Implement tokenizer
3. Add unit tests
4. Add golden cases:

   * repeated spaces
   * punctuation noise
   * mixed case
   * unicode accents if needed

### Done criteria

* `/v1/analyze` returns normalized string and tokens

---

## Phase 2 — Spell resolution via OpenSearch

### Goal

Support typo correction using OpenSearch native `did_you_mean` suggester / `fuzzy` queries — no custom spell engine.

### Tasks

1. Build OpenSearch client wrapper (interface for mocking)
2. Integrate `did_you_mean` suggester or `term`/`phrase` suggester
3. Add token-level correction strategy
4. Add thresholds:

   * min token length
   * skip numeric
   * skip likely SKU
   * skip stopwords
5. Add tests using mocked OpenSearch client

### Done criteria

* common typos corrected via OpenSearch native suggest
* no aggressive corruption of valid terms

---

## Phase 3 — Linguistic Expansion: Synonyms + Hypernyms + Compounds

### Goal

Add application-level synonym/hypernym expansion so "sneakers" also matches "trainers", "shoes" matches "footwear". Compound word split/join for "crewneck" → "crew neck".

**Why application-level, not OpenSearch synonym token filters?**

OpenSearch synonyms are index-time or analyzer-time and don't support weighted boosting per synonym type. The application-level approach lets us differentiate:

| Edge Type | Boost Weight |
|---|---|
| Exact | 1.0x |
| Synonym (SYN) | 0.9x |
| Hypernym (HYP) | 0.8x |
| Compound | 0.8x |
| Spell correction (SPELL1) | 0.6x |
| Spell correction (SPELL2) | 0.3x |

This weighted differentiation is what makes MXP's ranking work well — synonyms rank slightly below exact matches, hypernyms rank below synonyms.

**Data dependency**: Requires a linguistic dictionary index in OpenSearch (already exists from MXP's indexer). During transition, we keep using it. Phase 5 replaces the indexer.

### Tasks

1. Shingle generation: generate n-grams from tokens (1..maxShingleLength), respect phrase boundaries
2. LinguisticStep: query linguistic index for SYN/HYP/SW entries, create CONCEPT edges with normalization weights
3. Boost weighting: apply normalization penalties (SYN=0.9x, HYP=0.8x, SPELL1=0.6x, SPELL2=0.3x)
4. YAML config fallback for synonyms not in the linguistic index
5. Implement compound split/join rules
6. Add tests for rewrite order
7. Pipeline order: normalize → tokenize → spell → synonym/linguistic → compound

### Done criteria

* canonical rewrite is stable and deterministic
* synonym/hypernym matches rank below exact matches via boost weights

---

## Phase 4 — Concept recognition

### Goal

Extract structured concepts from query using OpenSearch `multi_match` with `cross_fields` + per-field boosts.

### Tasks

1. Build shingle generator
2. Implement concept search repository against OpenSearch (`multi_match` + boosts)
3. Define concept scoring scheme
4. Map concept hits to spans
5. Add market / locale filtering
6. Add max match count per span

### Done criteria

* concepts extracted for exact and alias cases
* results scored and attached to spans

---

## Phase 5 — Ambiguity resolution

### Goal

Prevent noisy overlapping concepts.

### Tasks

1. Implement overlap detection
2. Sort by:

   * longest span
   * highest score
   * highest weight
3. Remove losing overlaps
4. Add test cases:

   * overlapping phrases
   * exact vs synonym
   * long phrase vs short token

### Done criteria

* resolved concept set is clean and small

---

## Phase 6 — Comprehension rules

### Goal

Extract basic user intent beyond keywords.

### Tasks

1. Regex-based rule engine
2. Price rule extraction
3. Sort rule extraction
4. Static attribute filter extraction
5. Add tests and examples

### Done criteria

* phrases like `under 20`, `cheapest`, `vegan` return structured directives

---

## Phase 7 — Observability and robustness

### Goal

Make the MVP operable.

### Tasks

1. Add Prometheus metrics
2. Add per-step duration logging
3. Add request correlation ID
4. Add timeout handling
5. Add config validation
6. Add circuit-breaker-like fallback behavior only if necessary

   * for MVP, simple timeout/fallback is enough

### Done criteria

* service is debuggable in staging

---

## Phase 8 — Golden behavior suite

### Goal

Lock behavior.

### Tasks

1. Create `testdata/golden/*.json`
2. Add input/output golden tests
3. Add representative search queries from production samples
4. Add regression cases for typo, synonym, compound, ambiguity, price, sort

### Done criteria

* changes that affect behavior are visible immediately

---

# 12. Prompt Pack for Claude Code

Use this sequence. Do not ask Claude Code to solve everything in one prompt.

## Prompt 1 — Repository bootstrap

```text
Create a Go service named qus that exposes:
- GET /healthz
- POST /v1/analyze

Requirements:
- use idiomatic Go project structure
- use net/http or chi
- add config loading from yaml
- define request/response structs for AnalyzeRequest and AnalyzeResponse
- add structured logging
- return stubbed deterministic response for /v1/analyze
- include Makefile targets: run, test, fmt
- include unit tests for config loading and handler smoke test
```

## Prompt 2 — Normalize and tokenize

```text
Implement the normalization and tokenization pipeline for the Go QUS service.

Requirements:
- lowercase
- trim and collapse whitespace
- remove non-meaningful punctuation
- preserve token positions
- add unit tests
- add golden tests in testdata/golden for at least 10 representative cases
- update /v1/analyze to return normalizedQuery and tokens
Keep implementation simple and deterministic.
```

## Prompt 3 — OpenSearch spell integration

```text
Implement spell resolution for the Go QUS service using OpenSearch suggest APIs.

Requirements:
- create an interface for the OpenSearch client so it can be mocked in tests
- resolve typo corrections per token with configurable thresholds
- skip short tokens, numeric tokens, and likely SKUs
- if confidence is below threshold, keep original token
- update normalizedQuery / rewrites based on resolved tokens
- add unit tests and mocked OpenSearch tests
Do not implement custom edit distance logic yet.
```

## Prompt 4 — Synonyms and compounds

```text
Implement synonym expansion and compound word handling in the QUS pipeline.

Requirements:
- load synonyms and compound rules from yaml config
- support canonical replacement from variants
- support split and join compound rules
- keep only one canonical rewrite for MVP
- add deterministic ordering
- add unit tests and golden tests
Do not create multi-path branching.
```

## Prompt 5 — Concept recognition

```text
Implement concept recognition for the QUS pipeline.

Requirements:
- create shingles from tokens up to configurable max size
- search a concept index in OpenSearch via an interface
- support exact label matches and alias matches
- assign scores based on source: exact, synonym, spell, compound
- map concept matches to token spans
- filter by locale and market
- return concept matches in AnalyzeResponse
- add unit tests and mocked repository tests
Keep the implementation simple and production-readable.
```

## Prompt 6 — Ambiguity resolution

```text
Implement ambiguity resolution for concept matches.

Requirements:
- detect overlapping spans
- prefer longest span, then highest score, then highest weight
- remove losing overlaps
- add unit tests for overlapping phrase cases
- integrate into the QUS pipeline
Do not over-engineer this into a graph structure.
```

## Prompt 7 — Comprehension rules

```text
Implement a rule-based comprehension stage for QUS.

Requirements:
- extract price filters from phrases like "under 20", "less than 10", "cheaper than 5"
- extract sort directives from phrases like "cheapest", "lowest price", "newest"
- support a small config-driven rule set in yaml
- return filters and sort in AnalyzeResponse
- add unit tests and golden tests
Keep the rules deterministic and easy to extend.
```

## Prompt 8 — Observability and hardening

```text
Add production MVP hardening to the Go QUS service.

Requirements:
- per-step timing
- Prometheus metrics
- request correlation ID
- timeout handling for OpenSearch calls
- config validation
- health endpoint should verify config is loaded and dependencies can be initialized
- keep implementation lightweight
Add tests where practical.
```

---

# 13. Engineering Constraints for Claude Code

Include these directives in the implementation plan:

## Coding rules

* use idiomatic Go
* no reflection-heavy frameworks
* dependency injection via interfaces and constructors
* no global mutable state except config bootstrapping if unavoidable
* context propagation everywhere
* table-driven tests
* golden tests for behavior
* explicit errors, no panic in normal flow
* keep packages small and cohesive

## Non-goals

* do not port Java class hierarchy
* do not port Spring patterns into Go
* do not invent generic abstraction layers with no current need
* do not implement full graph semantics in MVP
* do not add premature concurrency

---

# 14. Acceptance Criteria

The MVP is complete when the following are true:

1. A request like:

```json
{
  "query": "cheap chiken burger under 20",
  "locale": "en-GB",
  "market": "uk"
}
```

returns:

* normalized query
* corrected typo
* token list
* at least one concept match
* extracted price filter

2. Service degrades safely if OpenSearch spell lookup fails

   * keeps original token
   * still returns usable response

3. Behavior is covered by:

* unit tests
* golden tests
* mocked OpenSearch integration tests

4. Output is stable enough for downstream search orchestration

---

# 15. Recommended MVP Milestones

| Milestone | Outcome                     | Duration |
| --------- | --------------------------- | -------- |
| M1        | API skeleton + tokenization | 2–3 days |
| M2        | spell + rewrite             | 3–5 days |
| M3        | synonyms + compounds        | 2–4 days |
| M4        | concept recognition         | 5–8 days |
| M5        | ambiguity + comprehension   | 4–6 days |
| M6        | tests + hardening           | 3–5 days |

Total MVP: **3–5 weeks** for one strong engineer.

---

# 16. Final Instruction Set for Claude Code

```text
Build an MVP Query Understanding Service in Go.

The service must:
- expose POST /v1/analyze
- normalize and tokenize raw queries
- perform typo correction using OpenSearch suggest APIs
- apply config-driven synonyms and compound word handling
- perform concept recognition against an OpenSearch-backed concept index
- resolve overlapping concept matches deterministically
- extract simple price and sort intent via rule-based comprehension
- return a structured AnalyzeResponse JSON
- include unit tests, golden tests, and mocked OpenSearch integration tests
- include Prometheus metrics, structured logging, and timeouts

Constraints:
- do not rebuild the Java graph model
- do not port Spring abstractions
- keep one canonical rewrite only
- keep the design idiomatic Go and production-readable
- prioritize deterministic behavior and clear package boundaries
```

## 17. Delivery Order

1. bootstrap
2. normalize/tokenize
3. spell
4. synonyms/compounds
5. concepts
6. ambiguity
7. comprehension
8. observability
9. golden regression suite

This is the correct MVP shape.
