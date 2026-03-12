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

**Key insight:** Spell correction should only fix typos (small edit distance), never replace valid words with different ones. Short words (≤5 chars) are especially prone to false corrections (e.g. "party"→"pasta" is only 2 edits) so they get a stricter threshold of 1 edit. Concept/alias mapping is a separate concern handled by the concept recognizer.

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
