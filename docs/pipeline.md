# QUS Pipeline Architecture

## Overview

QUS transforms raw user queries into structured search intent. Four pipelines are available, each suited to different trade-offs between latency, accuracy, and infrastructure cost.

| Pipeline | Endpoint | Strategy |
|---|---|---|
| **v1** (deterministic) | `/v1/analyze` | Sequential Go steps with OS dictionary lookups |
| **v2** (hybrid LLM) | `/v2/analyze` | LLM semantic parse + validation + concept resolution |
| **v3** (native OS) | `/v3/analyze` | Delegates spell/concept to OS fuzzy matching |
| **v4** (adaptive) | `/v4/analyze` | Token-count routing: short → v3, long → v2 LLM |

All pipelines produce the same `AnalyzeResponse` shape.

---

## v1 — Deterministic Pipeline

```
Input: "Cheap  CHIKEN  Burger under 20"  (locale: en-GB)
  │
  ▼
┌──────────────┐
│ 1. Normalize  │  lowercase, trim, collapse spaces, strip punctuation, unicode NFC, strip diacritics
└──────┬───────┘
       ▼
┌──────────────┐
│ 2. Tokenize   │  whitespace split, position tracking
└──────┬───────┘
       ▼
┌────────────────┐
│ 3. Comprehend  │  regex filter/sort extraction, selective token stripping
└──────┬─────────┘  "under 20" → price lt 20 (stripped), "cheap" → keyword (kept)
       ▼            normalizedQuery: "cheap chiken burger"
┌──────────────┐
│ 4. Spell      │  OS term suggester (suggest_mode=missing), Levenshtein guard
└──────┬───────┘  "chiken" → "chicken"
       ▼
┌──────────────┐
│ 5. Synonym    │  OS linguistic index (SYN only, HYP skipped)
└──────┬───────┘
       ▼
┌──────────────┐
│ 6. Compound   │  OS linguistic index (CMP): join "ice cream"→"icecream", split "crewneck"→"crew neck"
└──────┬───────┘
       ▼
┌──────────────┐
│ 7. Stopword   │  remove stopwords loaded from OS (type=SW), per-locale
└──────┬───────┘
       ▼
┌──────────────┐
│ 8. Concept    │  shingle → OS concept index, score by source (exact/synonym/compound/spell)
└──────┬───────┘
       ▼
┌──────────────┐
│ 9. Ambiguity  │  greedy non-overlapping: longest span → highest score → earliest position
└──────┬───────┘
       ▼
Output: AnalyzeResponse
```

### Step Details

#### 1. Normalizer (`normalizer.go`)

| Operation | Example |
|---|---|
| Lowercase | `BURGER` → `burger` |
| Trim + collapse spaces | `"  a    b  "` → `"a b"` |
| Strip noise punctuation | `chicken!!` → `chicken` (keeps hyphens, apostrophes) |
| Unicode NFC | Composed characters normalized |
| Strip diacritics | `café` → `cafe`, `über` → `uber` |

#### 2. Tokenizer (`tokenizer.go`)

Splits `NormalizedQuery` on whitespace. Each token gets a sequential `Position` index. Both `Value` and `Normalized` start identical — later steps may update `Normalized`.

#### 3. Comprehension Engine (`comprehension.go`)

Multi-locale regex extraction from `configs/comprehension.yaml` (8 languages: en, de, fr, nl, it, es, sv, da).

| Type | Pattern Example | Output |
|---|---|---|
| Prep time filter | `under 30 minutes` | `{field: "prep_time", operator: "lt", value: 30}` |
| Calorie filter | `low calorie` | `{field: "total_calories", operator: "lte", value: 400}` |
| Calorie tag filter | `calorie smart` | `{field: "tags", operator: "eq", value: "Calorie Smart"}` |
| Price filter | `under 20` | `{field: "price", operator: "lt", value: 20}` |
| Difficulty filter | `easy` | `{field: "difficulty_level", operator: "eq", value: "easy"}` |
| Servings filter | `for 4 people` | `{field: "servings", operator: "eq", value: 4}` |
| Sort directive | `cheapest` | `{field: "price", direction: "asc"}` |

Key behaviors:
- **Overlap detection**: tracks consumed character ranges — "under 15 minutes" matches prep_time only, not price
- **Rule ordering**: specific rules (prep_time, calories) before generic price
- **Selective token stripping**: numeric patterns ("under 10", "500 calories"), sort patterns ("cheapest"), and keyword filters with `strip: true` ("low calorie", "calorie smart") are stripped from tokens and query. Other keyword patterns ("quick", "healthy", "easy") are kept — they are meaningful search terms
- **Locale selection**: "en-GB" → lang prefix "en" → picks `en` rules

#### 4. Spell Resolver (`spell.go`)

Uses OpenSearch `term` suggester via the `SpellChecker` interface.

- **suggest_mode: `missing`** — only suggests when the term doesn't exist in the index. Prevents correcting valid words (e.g. "cheese" → "chinese").
- **Levenshtein guard**: AUTO-like scaling — ≤5 chars allows max 1 edit, 6+ chars allows max 2 edits. Rejects corrections that are too far from the original (e.g. "party" → "pasta" = 2 edits on 5-char word → rejected).
- **Skip rules**: tokens shorter than `min_token_length` (default: 4), numeric tokens, SKU patterns.
- **Graceful degradation**: if OpenSearch is unavailable, keeps original token and adds a warning.

#### 5. Synonym Expander (`synonym.go`)

Queries OpenSearch linguistic index for SYN entries with locale filtering.

- **HYP entries are skipped** — hypernyms go from broad to specific (e.g. "pasta" → "spaghetti") which narrows search incorrectly. Only true synonyms (SYN) replace tokens.
- No YAML fallback — all synonym data lives in OpenSearch.

#### 6. Compound Handler (`compound.go`)

Queries OpenSearch linguistic index for CMP entries.

| Operation | Example |
|---|---|
| **Join** | `ice cream` → `icecream` (adjacent tokens match a compound) |
| **Split** | `crewneck` → `crew neck` (single token matches a compound variant) |

Joins are applied first, then splits. Token positions are reindexed after changes.

#### 7. Stopword Filter (`stopword.go`)

Removes tokens whose normalized value is in the locale's stopword set. Stopwords are loaded once at startup from the OS linguistic index (type=SW) across all 26 supported locales.

#### 8. Concept Recognizer (`concept.go`)

1. Generates **shingles** (n-grams, 1..`shingle_max_size`) from tokens, longest first
2. Queries OpenSearch concept index using `multi_match` with `cross_fields` on `label` and `aliases`
3. **Word-count guard**: for multi-word shingles, rejects partial matches where the concept label has fewer words than the shingle (e.g. "spicy" from "spicy asian veggie" is skipped — it will be found by the 1-word shingle instead)
4. Filters by `locale` and `market`
5. Scores by source: exact > synonym > compound > spell
6. Caps at `max_matches_per_span` per shingle

#### 9. Ambiguity Resolver (`ambiguity.go`)

Greedy non-overlapping selection:
1. Sort concepts: longest span → highest score → earliest position
2. Iterate; skip any concept overlapping already-claimed positions
3. Result: clean, non-overlapping concept set

---

## v2 — Hybrid LLM Pipeline

```
Input: "cheap chicken burger under 20"  (locale: en-GB)
  │
  ▼
┌───────────────────────┐
│ 1. Normalize + Tokenize│  same as v1 steps 1-2
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 2. LLM Semantic Parse  │  Bedrock Nova: extract normalized query, filters, sorts, concepts
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 3. Validate            │  ground-truth filters against query text, validate normalized query
└──────────┬────────────┘  (Levenshtein check for hallucinated word substitutions)
           ▼
┌───────────────────────┐
│ 4. Resolve Concepts    │  match LLM candidate labels against OS concept index
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 5. Comprehension       │  strip filter/sort tokens from normalized query (same engine as v1)
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 6. Stopword Filter     │  remove filler words per locale
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 7. Assemble Response   │  merge tokens, concepts, filters, sort, rewrites
└──────────┬────────────┘
           ▼
Output: AnalyzeResponse
```

### Validation (`validation.go`)

- **Filter grounding**: LLM filter values must appear in the original query text. Prevents hallucinated values (e.g. "burger" → cuisine "American").
- **Normalized query validation**: `validateNormalizedQuery()` checks every word against the original. Words not in the original must be within edit distance (AUTO-like scaling). If any word is a hallucinated substitution, the entire query reverts to the original.
- **Allowed filters/sorts**: checked against `allowed_filters.yaml` and `allowed_sorts.yaml`.

### Fallback

When the LLM fails (timeout, parse error, etc.), v2 falls back to the deterministic path:
1. Run comprehension on the already-tokenized state
2. Run stopword filter
3. Return a `BuildFallbackResponse` with a warning

Controlled by `fail_open` config (if false, returns error instead of fallback).

### LLM Provider

AWS Bedrock (Nova model). `parseLLMOutput()` normalizes inconsistent field names (`rewrite` vs `rewrites`) and filter field aliases (`cuisine` → `recipe_cuisine`, `ingredient` → `ingredients`, etc.).

---

## v3 — Native OpenSearch Pipeline

```
Input: "chiken soup"  (locale: en-GB)
  │
  ▼
┌───────────────────────┐
│ 1. Normalize + Tokenize│  same as v1 steps 1-2
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 2. OS Fuzzy Spell      │  FuzzySuggest per token, Levenshtein guard (AUTO-like)
└──────────┬────────────┘  "chiken" → "chicken"
           ▼
┌───────────────────────┐
│ 3. Stopword Filter     │  same as v1
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 4. OS Fuzzy Concepts   │  FuzzySearchConcepts — OS does the fuzzy matching
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 5. Ambiguity           │  same as v1
└──────────┬────────────┘
           ▼
┌───────────────────────┐
│ 6. Comprehension       │  same engine as v1
└──────────┬────────────┘
           ▼
Output: AnalyzeResponse
```

v3 differs from v1 by delegating spell correction and concept matching to OS fuzzy queries instead of separate dictionary lookups for synonyms/compounds. This is simpler (fewer steps, fewer OS round-trips) but less controllable.

---

## v4 — Adaptive Pipeline

```
Input: "show me something easy for dinner"  (locale: en-GB)
  │
  ▼
┌───────────────────────┐
│ 1. Count non-stopword  │  lowercase + split + remove stopwords for locale
│    tokens              │  "show me something easy for dinner" → 4 non-stopword tokens
└──────────┬────────────┘
           │
     ┌─────┴──────┐
     │ ≥ threshold │  (default: 3)
     ▼            ▼
  ┌────────┐  ┌───────────────────────┐
  │ Run v3  │  │ Run v2 LLM Pipeline    │  direct to LLM for semantic understanding
  └────┬───┘  └──────────┬────────────┘
       │                 │
       ▼           ┌─────┴─────┐
    Return v3      │ success?  │
    result         ▼           ▼
               Return v2    Run v3
               result       (fallback)
```

### Routing Logic

v4 is a simple token-count router:

- **Short queries** (< `direct_llm_token_threshold` non-stopword tokens): Use v3 native OS pipeline. These are queries like "chicken", "pasta salad" — v3 handles them well with fuzzy spell correction and concept matching.
- **Longer queries** (≥ threshold): Skip v3 entirely and go straight to v2 LLM. These are queries like "healthy low carb meal prep", "show me something easy for dinner" — they benefit from semantic understanding.

The token count uses the same per-locale stopword sets as the pipeline. Words like "the", "for", "with" are excluded from the count.

### Configuration

In `configs/qus.yaml`:

```yaml
adaptive:
  direct_llm_token_threshold: 3  # 0 to disable (v3 only)
```

### Behavior when v2 is unavailable

If the LLM is not configured (`v2 == nil`) or threshold is 0, v4 always uses v3 — it degrades gracefully to a pure v3 pipeline. If v2 returns empty tokens and no filters (LLM failure with `fail_open`), v4 falls back to v3.

---

## Pipeline State

All steps operate on a shared `QueryState`:

```go
type QueryState struct {
    OriginalQuery   string
    NormalizedQuery string
    Locale          string
    Market          string
    Tokens          []Token
    Concepts        []ConceptMatch
    Filters         []Filter
    Sort            *SortSpec
    Warnings        []string
    Debug           []StepDebug
}
```

## Configuration Files

| File | Used By | Purpose |
|---|---|---|
| `configs/qus.yaml` | v1, v3, v4 | Pipeline settings (spell thresholds, concept limits) |
| `configs/comprehension.yaml` | v1, v3, v4 | Multi-locale filter/sort regex rules (en, de, fr, nl, it, es, sv, da) |
| `configs/allowed_filters.yaml` | v2, v4* | Allowed filter fields, types, operators |
| `configs/allowed_sorts.yaml` | v2, v4* | Allowed sort fields and directions |
| `configs/llm_prompt.txt` | v2, v4* | LLM system prompt template |

*v4 uses these only when escalating to v2.

## OpenSearch Dependencies

| Index | Used By | Purpose |
|---|---|---|
| `concepts_{locale}` | v1 Concept, v2 ConceptResolver, v3 FuzzyConcepts | Concept search with locale-aware stemming |
| `linguistic_{locale}` | v1 Spell/Synonym/Compound/Stopword, v3 Spell | SYN/HYP/CMP/SW dictionary lookups |

## Running Locally

```bash
# Start OpenSearch
docker compose up -d

# Seed data
./scripts/seed-opensearch.sh      # linguistic + concept indexes
./scripts/seed-compounds.sh       # CMP entries from TSV files

# Run QUS
make run

# Test endpoints
curl -s localhost:8080/healthz
curl -s -X POST localhost:8080/v1/analyze \
  -H 'Content-Type: application/json' \
  -d '{"query":"cheap chiken burger under 20","locale":"en-GB","market":"uk"}'
```
