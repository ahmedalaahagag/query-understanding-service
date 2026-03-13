# QUS — Problems & Solutions

## 1. YAML Data Files Don't Scale

**Problem:** Synonyms and compounds were stored as YAML config files (`synonyms.{locale}.yaml`, `compounds.{locale}.yaml`). This meant:
- Adding new synonyms/compounds required a code deploy
- No way to update dictionary data without restarting the service
- YAML files grew large and hard to maintain per locale
- Inconsistent with the concept index which was already in OpenSearch

**Solution:** Moved all dictionary data to OpenSearch's linguistic index (`linguistic_{locale}`).
- Synonyms → type `SYN` entries: `{"term": "burger", "variant": "hamburger", "type": "SYN"}`
- Compounds → type `CMP` entries: `{"term": "icecream", "variant": "ice cream", "type": "CMP"}`
- Deleted `configs/synonyms.*.yaml` and `configs/compounds.*.yaml`
- Refactored `SynonymExpander` and `CompoundHandler` to use OS-backed `LinguisticLookup` and `CompoundLookup` interfaces
- YAML configs retained only for: `qus.yaml` (pipeline settings), `comprehension.yaml` (regex rules), v2 LLM configs

**Files changed:** `internal/domain/pipeline/synonym.go`, `compound.go`, `pkg/config/domain.go`, `pkg/analyzer/analyzer.go`, `cmd/http.go`

---

## 2. Compound Data Management at Scale

**Problem:** After moving compounds to OpenSearch, we needed a way to manage 1,700+ compound entries across 8 locales without embedding them all in `seed-opensearch.sh`.

**Solution:** Created a standalone compound management system:
- TSV files in `scripts/compound-data/{locale}.tsv` — human-readable, easy to edit, supports comments with `#`
- Dedicated `scripts/seed-compounds.sh` script that reads TSV files and bulk-indexes CMP entries
- Can seed all locales or a single locale: `./scripts/seed-compounds.sh [URL] [locale]`
- 1,767 entries across 8 locales: en_gb (472), en_us (301), de_de (234), nl_nl (177), sv_se (156), fr_fr (156), es_es (136), it_it (135)

**Files created:** `scripts/seed-compounds.sh`, `scripts/compound-data/*.tsv`

---

## 3. Golden Tests Broke After Removing YAML Synonyms

**Problem:** The e2e golden tests used mock OS backends but some test cases relied on synonym expansions that were previously loaded from YAML (e.g. "veggie" → "vegetarian", "coke" → "coca cola"). After removing YAML, these tests failed because the mocks didn't have the SYN entries.

**Solution:** Added the former YAML synonym entries to the `e2eLinguisticLookup` mock in `e2e_golden_test.go`. The mock now returns the same data that was previously in YAML, but via the OS lookup interface.

**Files changed:** `internal/domain/pipeline/e2e_golden_test.go`

---

## 4. Comprehension Engine Token Stripping in Golden Tests

**Problem:** After comprehension was properly wired, golden test expected values were wrong. The comprehension engine strips consumed filter/sort tokens (e.g. "cheapest", "under 20") from `normalizedQuery` and `tokens`, but the golden test JSON files still had these tokens in the expected output.

**Examples:**
- "cheapest chicken burger under 20" → expected normalizedQuery was the full string, but actual was "chicken burger" (after stripping "cheapest" and "under 20")
- "pizza under 15" → expected "pizza under 15", actual "pizza"

**Solution:** Updated all affected golden test JSON files to reflect post-comprehension state:
- `normalizedQuery`: stripped of consumed tokens
- `tokens`: only non-consumed tokens with re-indexed positions
- `rewrites`: contains the stripped normalizedQuery (since it differs from originalQuery)
- `concepts.start/end`: kept at pre-comprehension positions (concepts are matched before stripping)

**Files changed:** `testdata/golden/e2e_combined_typo_price.json`, `e2e_price_filter.json`, `e2e_sort_directive.json`, `e2e_synonym_expansion.json`

---

## 5. Native Pipeline Test Expected Pre-Comprehension Output

**Problem:** The v3 native pipeline test for "chicken under 10" expected `normalizedQuery: "chicken under 10"`, but comprehension strips "under 10" as a price filter.

**Solution:** Changed the assertion to expect `normalizedQuery: "chicken"`.

**Files changed:** `internal/domain/native/pipeline_test.go`

---

## 6. Concept "veggies" Not Resolving (No Stemming)

**Problem:** Querying "veggies" returned no concept match even though the concept index had `diet-vegetarian` with alias `"veggie"`. The concept index used the default `standard` analyzer which only does tokenization and lowercasing — **no stemming**. So "veggies" (plural) didn't match "veggie" (singular).

This affected all languages, not just English. German compounds, French plurals, etc. all had the same issue.

**Why stemmer?** A stemmer reduces words to their root form at both index and query time:
- Index time: "veggie" → stem "veggi" (stored)
- Query time: "veggies" → stem "veggi" (searched) → match!

Alternatives rejected:
- **Manual aliases** (add "veggies" alongside "veggie"): Doesn't scale — every word needs every inflection across 10+ languages
- **Fuzzy matching** (`fuzziness: AUTO`): v3 already uses this but it matches by edit distance — too loose, causes false positives ("for" → "fork", "somthing" → "indian")
- **Lemmatizer**: Not available as an OpenSearch token filter. Stemmers are the standard ES/OS approach

**Solution:** Added locale-aware stemming analyzers to the concept index:
- Each `concepts_{locale}` index now gets a custom analyzer with the appropriate language stemmer
- Locale mapping: `en_*` → english, `de_*` → german, `fr_*` → french, `nl_*` → dutch, `it_*` → italian, `es_*` → spanish, `sv_*` → swedish, `da_*` → danish, `nb_*` → norwegian
- CJK and unknown locales use built-in analyzers without custom stemmer (stemmer filter not available)
- Applied to `label` and `aliases` fields in the concept index mapping only — not to `keyword` fields like `id` or `field`
- Zero maintenance: new aliases automatically match their inflected forms

**Files changed:** `scripts/seed-opensearch.sh`

---

## 7. False Positive Concept Matches from Stopwords

**Problem:** Stopwords like "for", "with", "something" were being sent to concept search and causing false positives:
- "for" → fuzzy-matched "for kids" alias → resolved to "family friendly" concept
- "somthing" → fuzzy-matched "indian" (edit distance)

The v2 pipeline already had stopword filtering, but v1 and v3 did not.

**Solution:** Created `StopwordFilter` pipeline step (`internal/domain/pipeline/stopword.go`):
- Removes tokens whose normalized value is in the stopword set
- Re-indexes remaining token positions
- Rebuilds the normalized query
- Stopwords loaded once from OS linguistic index (type=SW) at startup
- Shared across all 3 pipelines

Pipeline placement (before concept recognition):
- v1: ... → Compound → **Stopword** → Concept → Ambiguity
- v3: ... → Spell → **Stopword** → Concept → Ambiguity → Comprehension
- v2: already had filtering, also wired missing comprehension and stopwords

**Files created:** `internal/domain/pipeline/stopword.go`, `stopword_test.go`
**Files changed:** `pkg/analyzer/analyzer.go`, `cmd/http.go`, `internal/domain/native/pipeline.go`

---

## 8. Nova LLM Inconsistent Field Names

**Problem:** AWS Bedrock Nova model sometimes returns `"rewrite"` (singular) instead of `"rewrites"` (plural), and sometimes returns rewrites as a string or object instead of an array. Direct `json.Unmarshal` into the typed struct would fail or lose data.

**Solution:** Added `parseLLMOutput()` function that:
1. Unmarshals raw JSON into a `map[string]json.RawMessage` (flexible)
2. Normalizes `"rewrite"` → `"rewrites"` key
3. Checks if `"rewrites"` value is an array — if not, drops it to avoid unmarshal errors
4. Re-marshals the normalized map and unmarshals into the typed `LLMParseResult`

**Files changed:** `internal/infra/bedrock/client.go`

---

## 9. v2 Fallback Path Missing Comprehension

**Problem:** When the LLM failed and v2 fell back to the deterministic pipeline, the comprehension step was not running. This meant filter/sort tokens (e.g. "cheap", "under 20") remained in the normalized query and token list, polluting the search query sent to the orchestrator.

**Solution:** Wired the comprehension engine into the v2 fallback path so it runs before building the fallback response. Also added stopword filtering on the fallback path.

**Files changed:** `internal/domain/hybrid/pipeline.go`

---

## 10. v1 Pipeline Step Order (Comprehension Placement)

**Problem:** In `cmd/http.go`, comprehension was the LAST step in the v1 pipeline (after concept recognition). This meant concepts were matched on the full query including filter/sort tokens, and positions were wrong after stripping.

In `pkg/analyzer/analyzer.go`, comprehension was correctly placed EARLY (after tokenizer, before spell). This inconsistency caused different behavior between HTTP and library usage.

**Solution:** Aligned `cmd/http.go` with `analyzer.go` — comprehension runs early in v1 (after tokenizer, before spell) so filter/sort tokens are stripped before any linguistic processing or concept matching.

**Files changed:** `cmd/http.go`

---

## 11. CJK Analyzer Doesn't Support Stemmer Filter

**Problem:** When adding locale-aware analyzers, the `ja_jp` (Japanese) locale was mapped to `"cjk"` analyzer. The initial implementation tried to create a custom analyzer with a CJK stemmer filter, but OpenSearch doesn't support a stemmer for CJK — index creation returned `acknowledged=null`.

**Solution:** Added a branch in `concept_mapping()`: CJK and `standard` analyzers use the built-in analyzer directly (set on fields via `"analyzer": "cjk"`), while all other languages use a custom analyzer with `standard` tokenizer + `lowercase` filter + language-specific stemmer filter.

**Files changed:** `scripts/seed-opensearch.sh`

---

## 12. Hypernym Replacement Narrows Search ("pasta" → "ravioli")

**Problem:** All 3 pipelines were replacing broad food category words with specific types. For example, searching "pasta" would return results for "ravioli" instead.

**Root causes:**
1. **v1 synonym expander** treated HYP (hypernym) entries the same as SYN (synonym). The linguistic index has entries like `{variant: "pasta", term: "spaghetti", type: "HYP"}`, meaning "pasta is a variant of spaghetti". The expander replaced "pasta" with "spaghetti" — narrowing the search incorrectly.
2. **v2 LLM** hallucinated word substitutions in `normalizedQuery`. The prompt said "fix typos" but Nova replaced correctly spelled words like "pasta" with "ravioli". The validator checked filters/concepts but did NOT validate the normalized query.
3. **v3 native spell corrector** used `FuzzySuggest` which returns the concept **label**, not the matched text. If "pasta" matched a concept alias, it returned the label (e.g. "ravioli") as a "correction" even though it wasn't a typo.

**Fixes:**
1. **v1:** Skip HYP-type replacements in synonym expander. Only SYN entries replace tokens. Add Levenshtein guard in spell checker with AUTO-like scaling (≤5 chars → max 1 edit, 6+ chars → max 2 edits). (`internal/domain/pipeline/synonym.go`, `spell.go`)
2. **v2:** Add `validateNormalizedQuery()` — compares positionally when word counts match, set-based when they differ. Uses same AUTO-like edit distance scaling. Also strengthened LLM prompt to prohibit word substitutions. (`internal/domain/hybrid/validation.go`, `configs/llm_prompt.txt`)
3. **v3:** Add Levenshtein distance check in `NativeSpellCorrector` with AUTO-like scaling. (`internal/domain/native/spell.go`)

**Levenshtein implementation:** We implemented a standard dynamic-programming Levenshtein distance function (`levenshtein(a, b string) int`) in each package that needs it (`pipeline/spell.go`, `native/spell.go`, `hybrid/validation.go`). The function uses two rows (`prev`/`curr`) for O(n) space. We chose to inline rather than share via a utility package — three identical ~20-line functions is simpler than adding a shared dependency for a single function (merciless simplification).

**AUTO-like scaling:** Inspired by OpenSearch's `fuzziness: AUTO` behavior — short words tolerate fewer edits because even 1-2 edits can produce a completely different word:
- ≤5 chars: max 1 edit (e.g. "party"→"pasta" = 2 edits → rejected)
- 6+ chars: max 2 edits (e.g. "chiken"→"chicken" = 1 edit → accepted)

**Key insight:** Spell correction should only fix typos (small edit distance), never replace valid words with different ones. Short words (≤5 chars) are especially prone to false corrections so they get a stricter threshold of 1 edit. Concept/alias mapping is a separate concern handled by the concept recognizer.

---

## 13. Go RE2 Doesn't Support Lookaheads

**Problem:** When extending the comprehension engine to support multiple filter types (price, prep_time, calories), we needed price patterns like `under \d+` to NOT match when followed by "minutes" or "calories". The initial approach used negative lookaheads (`(?!\s*(?:min|minute))`), but Go's RE2 engine does not support lookaheads — compilation failed with `error parsing regexp: invalid or unsupported Perl syntax`.

**Solution:** Two-part approach:
1. **Rule ordering**: More specific rules (prep_time `under \d+ minutes`, calories `under \d+ cal`) come before the generic price rule (`under \d+`) in the config
2. **Overlap detection**: Track consumed character ranges with a `[]bool` slice. When a rule matches, mark its character range as consumed. Later rules that overlap an already-consumed range are skipped.

This eliminates the need for lookaheads entirely and works across all locales.

**Files changed:** `internal/domain/pipeline/comprehension.go`, `configs/comprehension.yaml`

---

## 14. Bulk Insert to OpenSearch Silently Drops All Documents

**Problem:** The `copy-indexes.sh` script scrolled through all source documents and sent bulk insert requests to the destination cluster. Progress counters showed all documents copied, but `_count` returned 0. No errors were reported.

**Root cause:** The `dst_curl` helper function hardcoded `Content-Type: application/json`. The bulk insert call used `dst_curl` and added a second header `-H "Content-Type: application/x-ndjson"`, resulting in **duplicate Content-Type headers**. OpenSearch used the first one (`application/json`), silently failed to parse the ndjson payload, and returned an empty response (`{"errors": null, "items": []}`) with no error indication.

**Solution:** Use `curl` directly for bulk requests instead of the `dst_curl` helper, setting only `Content-Type: application/x-ndjson`.

**Key insight:** When curl sends duplicate headers of the same name, the server may use either one. Always ensure bulk API calls use exactly one `Content-Type: application/x-ndjson` header. Also add an explicit `_refresh` call before verification counts, since auto-refresh may not have run yet for large imports.

**Files changed:** `scripts/copy-indexes.sh`

---

## 15. Spell Checker Corrects Valid Words ("cheese" → "chinese")

**Problem:** Searching "mac and cheese" produced `normalizedQuery: "mac chinese"`. The spell checker was replacing "cheese" with "chinese" because:
1. The OpenSearch term suggester used `suggest_mode: "popular"`, which returns suggestions that are **more frequent** than the original term — even when the original exists in the index
2. "chinese" is a standalone concept label (cuisine) with high doc frequency; "cheese" only appears as part of multi-word labels ("cream cheese", "cheddar cheese")
3. `levenshtein("cheese", "chinese") = 2`, and "cheese" is 6 chars → maxEdits=2 → correction accepted

The `popular` mode is designed for "did you mean?" scenarios where the user might have typed a rare word. But in a food search context, "cheese" is a perfectly valid search term — it just happens to appear as part of compound labels rather than a standalone one.

**First attempt (insufficient):** Changed `suggest_mode` from `"popular"` to `"missing"`. The `missing` mode only suggests when the original term does not exist in the index. Since "cheese" appears in multi-word labels ("cream cheese"), it should have been considered "existing". However, the term suggester operates **per shard** — with 5 shards and only 2 docs containing "cheese", some shards had no "cheese" token at all, so `missing` mode still returned "chinese" on those shards.

**Final solution:** Added a `termExistsInConcepts` pre-check in the `Suggest` method. Before querying the term suggester, it runs a quick `multi_match` search against `label` and `aliases` fields in the concept index. If the original token appears in any concept (even as part of a multi-word label), no suggestions are returned — the word is valid.

This works regardless of shard count because `_search` aggregates results across all shards, unlike the term suggester which operates per-shard.

Reverted `suggest_mode` back to `"popular"` since the existence check now handles the filtering.

**Why not other fixes?**
- **`suggest_mode=missing`**: Doesn't work reliably on multi-shard indices when the term is sparse (per-shard term frequency)
- **Reduce shard count to 1**: Would fix `missing` mode but requires reindexing and affects all queries
- **Add "cheese" as a standalone concept**: Doesn't scale — every valid English word would need adding
- **Blocklist common words**: Maintenance burden, language-specific, doesn't generalize

**Files changed:** `internal/infra/opensearch/client.go`

---

## 16. When to Escalate from Deterministic to LLM

**Problem:** The v3 native pipeline is fast (~10–20ms) but can't handle conversational or ambiguous queries ("show me something easy for dinner", "healthy meal prep ideas"). The v2 LLM pipeline understands intent but costs ~200–500ms and requires an LLM provider. Running v2 for every query wastes latency and money on simple queries that v3 handles perfectly.

We needed a way to automatically decide: is v3's output good enough, or should we escalate to the LLM?

**Solution:** Built the v4 adaptive pipeline (`internal/domain/adaptive/`) that runs v3 first, scores its output, and only escalates to v2 when needed.

### Complexity Scorer (`scorer.go`)

Evaluates v3 output on four weighted signals that together produce a score from 0 (simple) to 1 (complex):

| Signal | Weight | What it measures | Default threshold |
|---|---|---|---|
| Token coverage | 0.4 | Fraction of tokens not matched by any concept | `max_uncovered_ratio: 0.5` |
| Concept confidence | 0.2 | Average concept match score too low | `min_concept_score: 0.7` |
| Spell corrections | 0.2 | Number of tokens that were spell-corrected | `max_spell_corrections: 2` |
| Conversational patterns | 0.2 | Regex match on the original query | `min_tokens_for_conversational: 5` |

**Token coverage** (strongest signal at 0.4 weight): If v3 matched concepts for most tokens, it understood the query. If half the tokens are uncovered, something is ambiguous or unknown. Scales linearly from 0 to the threshold — 25% uncovered with a 50% threshold contributes `0.4 * 0.5 = 0.2` to the score.

**Concept confidence**: Low average scores suggest fuzzy/uncertain matches — v3 found *something* but isn't confident. Binary: below threshold adds full 0.2.

**Spell corrections**: A few corrections (1–2) are normal typo fixes. Above the threshold suggests garbled input where the LLM's broader language understanding helps. Scales linearly up to the threshold.

**Conversational patterns**: Regex detects phrases like "show me", "i want", "something", "looking for", "give me", "can you", "anything", "recommend". These are natural language constructs that the deterministic pipeline strips as noise but the LLM can interpret semantically. Only checked when token count ≥ 5 (short queries with one matching word aren't truly conversational).

### Escalation Triggers

Escalate if **any** of these is true:
1. **Composite score > 0.5** — multiple weak signals compound into a complex query
2. **Conversational phrasing** — hard trigger regardless of score (deterministic pipelines can't parse "show me something easy")
3. **v3 understood nothing** — 3+ tokens with zero concepts AND zero filters (v3 produced no useful structure)

### Why these signals?

**Token coverage** was chosen as the heaviest signal (0.4) because it directly measures how much of the query v3 could structure. A query where 80% of tokens are concepts is well-understood; one where 80% are uncovered is opaque.

**Conversational patterns** get a hard trigger (not just score weight) because these queries fundamentally require semantic understanding — no amount of fuzzy matching will parse "i want something quick and healthy" into useful structure. The deterministic pipeline would strip "i want" and "something" as stopwords, losing the user's actual intent.

**The 3+ token zero-concept zero-filter trigger** catches edge cases where the query is meaningful but entirely outside the concept/comprehension vocabulary. "meal prep ideas" has clear intent but v3 might not have concepts for any of those terms.

### Fallback Behavior

- If v2 is not configured (`LLM.Enabled=false`), v4 always returns v3's result — graceful degradation
- If v2 fails (LLM timeout, parse error) or returns empty tokens, v4 falls back to v3's result
- The v3 result is never discarded — it's always available as a safety net

### Tuning

All thresholds are configurable via `ScorerConfig`. To make the scorer more aggressive (escalate more often), lower `MaxUncoveredRatio` or `MinConceptScore`. To reduce LLM usage, raise the thresholds or increase the score threshold in the escalation check.

**Example scoring:**

| Query | Coverage | Concepts | Spells | Conversational | Score | Escalate? |
|---|---|---|---|---|---|---|
| "chicken" | 1.0 | 0.95 avg | 0 | no | 0.0 | No |
| "pasta under 30 min" | 0.5 | 0.9 avg | 0 | no | 0.2 | No |
| "chikken brest recipee" | 0.67 | 0.8 avg | 3 | no | ~0.33 | No |
| "show me something easy" | 0.17 | 0.9 avg | 0 | yes | ~0.53 | **Yes** (conversational) |
| "meal prep ideas" | 0.0 | — | 0 | no | 0.4 | **Yes** (0 concepts + 0 filters) |
| "healthy quick dinner" | 0.0 | — | 0 | no | 0.4 | **Yes** (0 concepts + 0 filters) |

**Files created:** `internal/domain/adaptive/scorer.go`, `scorer_test.go`, `pipeline.go`, `pipeline_test.go`
**Files changed:** `pkg/analyzer/analyzer.go`, `internal/application/routes/routes.go`, `cmd/http.go`
