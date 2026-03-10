# QUS Pipeline Architecture

## Overview

The QUS pipeline transforms raw user queries into structured search intent. It uses a sequential step model — each step receives a mutable `QueryState` and enriches it.

```
Input: "Cheap  CHIKEN  Burger under 20"
  │
  ▼
┌─────────────┐
│ 1. Normalize │  lowercase, trim, collapse spaces, strip punctuation, unicode NFC, strip diacritics
└──────┬──────┘
       ▼
┌─────────────┐
│ 2. Tokenize  │  whitespace split, position tracking
└──────┬──────┘
       ▼
┌─────────────┐
│ 3. Spell     │  OpenSearch term suggester, per-token correction, confidence threshold
└──────┬──────┘
       ▼
┌─────────────┐
│ 4. Synonym   │  OpenSearch linguistic index (SYN/HYP) → YAML config fallback
└──────┬──────┘
       ▼
┌─────────────┐
│ 5. Compound  │  join multi-token compounds, split single compounds
└──────┬──────┘
       ▼
┌─────────────┐
│ 6. Concept   │  shingle → OpenSearch concept index, score by source
└──────┬──────┘
       ▼
┌─────────────┐
│ 7. Ambiguity │  remove overlapping concepts (longest span > highest score > earliest)
└──────┬──────┘
       ▼
┌───────────────┐
│ 8. Comprehend │  regex price/sort/filter extraction from config rules
└──────┬────────┘
       ▼
Output: AnalyzeResponse JSON
```

## Pipeline State

All steps operate on a shared `QueryState`:

```go
type QueryState struct {
    OriginalQuery   string
    NormalizedQuery string
    Tokens          []Token
    Concepts        []ConceptMatch
    Filters         []Filter
    Sort            *SortSpec
    Warnings        []string
    Debug           []StepDebug
}
```

## Step Details

### 1. Normalizer (`normalizer.go`)

| Operation | Example |
|---|---|
| Lowercase | `BURGER` → `burger` |
| Trim + collapse spaces | `"  a    b  "` → `"a b"` |
| Strip noise punctuation | `chicken!!` → `chicken` (keeps hyphens, apostrophes) |
| Unicode NFC | Composed characters normalized |
| Strip diacritics | `cafe` ← `café`, `uber` ← `über` |

### 2. Tokenizer (`tokenizer.go`)

Splits `NormalizedQuery` on whitespace. Each token gets a sequential `Position` index. Both `Value` and `Normalized` start identical — later steps may update `Normalized`.

### 3. Spell Resolver (`spell.go`)

Uses OpenSearch `term` suggester via the `SpellChecker` interface.

**Skip rules:**
- Token shorter than `min_token_length` (default: 4)
- Numeric tokens (`123`, `45.99`)
- SKU patterns (`AB1234`)

**Threshold:** Only accepts suggestions with `score >= confidence_threshold` (default: 0.85).

**Graceful degradation:** If OpenSearch is unavailable, keeps original token and adds a warning.

### 4. Synonym Expander (`synonym.go`)

Two-tier expansion strategy:

1. **OpenSearch linguistic index** (primary) — queries for SYN/HYP entries with locale filtering
2. **YAML config fallback** — static `variant → canonical` mapping from `configs/synonyms.en-GB.yaml`

Linguistic index takes priority over config. This supports weighted boost differentiation:

| Edge Type | Boost Weight |
|---|---|
| Exact | 1.0x |
| Synonym (SYN) | 0.9x |
| Hypernym (HYP) | 0.8x |
| Compound | 0.8x |
| Spell (SPELL1) | 0.6x |
| Spell (SPELL2) | 0.3x |

**Why not OpenSearch synonym token filters?** They're index/analyzer-time and don't support per-type weighted boosting. Application-level expansion lets synonyms rank below exact matches.

### 5. Compound Handler (`compound.go`)

Two operations from `configs/compounds.en-GB.yaml`:

| Operation | Example |
|---|---|
| **Join** | `ice cream` → `icecream` |
| **Split** | `crewneck` → `crew neck` |

Joins are applied first, then splits. Token positions are reindexed after changes.

### 6. Concept Recognizer (`concept.go`)

1. Generates **shingles** (n-grams, 1..`shingle_max_size`) from tokens, longest first
2. Queries OpenSearch concept index using `multi_match` with `cross_fields` on `label` and `aliases`
3. Filters by `locale` and `market`
4. Scores by source: exact=1.0, synonym=0.9, compound=0.8, spell=0.7
5. Caps at `max_matches_per_span` per shingle

### 7. Ambiguity Resolver (`ambiguity.go`)

Greedy non-overlapping selection:

1. Sort concepts: longest span → highest score → earliest position
2. Iterate sorted list; skip any concept overlapping already-claimed positions
3. Result: clean, non-overlapping concept set

### 8. Comprehension Engine (`comprehension.go`)

Regex-based extraction from `configs/comprehension.en-GB.yaml`:

| Type | Pattern Example | Output |
|---|---|---|
| Price filter | `under 20`, `less than 10`, `cheaper than 5.99` | `{field: "price", operator: "lt", value: 20}` |
| Sort directive | `cheapest`, `lowest price` | `{field: "price", direction: "asc"}` |
| Sort directive | `newest`, `most recent` | `{field: "created_at", direction: "desc"}` |

## Configuration Files

| File | Purpose |
|---|---|
| `configs/qus.yaml` | Pipeline settings (enabled steps, spell thresholds, concept limits) |
| `configs/synonyms.en-GB.yaml` | Static synonym fallback (`variant → canonical`) |
| `configs/compounds.en-GB.yaml` | Compound split/join rules |
| `configs/comprehension.en-GB.yaml` | Price/sort regex extraction rules |

## OpenSearch Dependencies

| Index | Used By | Purpose |
|---|---|---|
| Concepts (`QUS_OPENSEARCH_INDEX`) | Spell, Concept | Spell suggest + concept search |
| Linguistic (`QUS_OPENSEARCH_LINGUISTIC_INDEX`) | Synonym | SYN/HYP dictionary lookup |

## Running Locally

```bash
# Start OpenSearch
docker compose up -d

# Run QUS
make run

# Test
curl -s localhost:8080/healthz
curl -s -X POST localhost:8080/v1/analyze \
  -H 'Content-Type: application/json' \
  -d '{"query":"cheap chiken burger under 20","locale":"en-GB","market":"uk"}'
```
