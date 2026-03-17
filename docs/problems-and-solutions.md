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

## 16. When to Route to LLM vs Deterministic

**Problem:** The v3 native pipeline is fast (~10–20ms) but can't handle conversational or multi-concept queries ("show me something easy for dinner", "healthy low carb meal prep"). The v2 LLM pipeline understands intent but costs ~200–500ms. Running v2 for every query wastes latency and money on simple queries that v3 handles perfectly.

**Original approach:** Built a complexity scorer that ran v3 first, then scored its output on four weighted signals (token coverage, concept confidence, spell corrections, conversational patterns) to decide whether to escalate to v2. This was over-engineered — the scorer added complexity without clear benefit over a simpler heuristic.

**Simplified solution:** Token-count routing. Count non-stopword tokens in the query. Short queries (< threshold) go to v3. Longer queries go straight to v2 LLM — no v3 run, no scoring.

The insight: query length after stopword removal is a strong enough proxy for complexity. Single-word queries ("chicken") and two-word queries ("pasta salad") are what v3 excels at — fuzzy spell correction + concept matching. Three+ word queries ("healthy meal prep", "show me something easy for dinner") benefit from semantic understanding.

### Configuration

In `configs/qus.yaml`:

```yaml
adaptive:
  direct_llm_token_threshold: 3  # 0 to disable (v3 only)
```

### Routing examples

| Query | Non-stopword tokens | Route |
|---|---|---|
| "chicken" | 1 | v3 |
| "pasta salad" | 2 | v3 |
| "healthy meal prep" | 3 | v2 LLM |
| "show me something easy for dinner" | 4 (show, something, easy, dinner) | v2 LLM |

### Fallback

- If v2 is not configured (`LLM.Enabled=false`) or threshold is 0, v4 always uses v3
- If v2 returns empty tokens and no filters, v4 falls back to v3

**Files changed:** `internal/domain/adaptive/pipeline.go`, `pipeline_test.go`, `pkg/config/domain.go`, `configs/qus.yaml`
**Files deleted:** `internal/domain/adaptive/scorer.go`, `scorer_test.go`

---

## 17. Spell Correction First-Letter Guard

**Problem:** The spell corrector changed "dinner" → "ginger" (edit distance 2, within the AUTO fuzziness limit for 6-char words). The concept index doesn't contain "dinner" so `suggest_mode: missing` returns "ginger" as a fuzzy match. This caused "show me something easy for dinner" to return ginger recipes instead of easy dinner recipes.

**Root cause:** The Levenshtein guard (≤5 chars → 1 edit, 6+ → 2 edits) was necessary but not sufficient. "dinner" → "ginger" is only 2 substitutions (d→g, n→g) so it passed the distance check, despite being an obviously wrong correction.

**Solution:** Added a first-letter guard to both v1 and v3 spell correctors. Valid typos almost never change the first character — users know what letter a word starts with, even when they misspell the middle. If `corrected[0] != token[0]`, the correction is rejected.

```go
if corrected[0] != tok.Value[0] {
    continue
}
```

**Why not add "dinner" to the concept index?** "Dinner" isn't a food concept — it's a meal occasion. Adding non-concepts to the concept index to prevent spell corrections is a data integrity smell. The spell corrector should be smart enough to reject bad suggestions on its own.

**Why not tighten the Levenshtein limit?** Reducing max edits from 2 to 1 for 6-char words would break valid corrections like "chiken" → "chicken" (2 edits). The first-letter check is a more targeted constraint.

**Files changed:** `internal/domain/native/spell.go`, `internal/domain/pipeline/spell.go`

## 18. LLM Filter Field Names Don't Match Product Index

**Problem:** The LLM returns filter field names like `meal_type`, `cooking_method`, `cuisine`, `ingredient`, `category` — but the product index has different field names (`recipe_cuisine`, `ingredients`, `categories`) or doesn't have the field at all (`meal_type`, `cooking_method`). This caused 0-hit searches because OpenSearch can't filter on non-existent fields.

**Example:** "show me something easy for dinner" → LLM returns `meal_type eq "dinner"` + `difficulty_level eq "easy"`. The `meal_type` field doesn't exist in the product index → 0 results, even though `difficulty_level` alone would have worked.

**Solution (three layers):**
1. **Remove phantom fields from `allowed_filters.yaml`**: Deleted `meal_type` and `cooking_method` — these don't exist in the product index. Meal occasions and cooking methods are stored in the `tags` field instead.
2. **Rename mismatched fields**: `cuisine` → `recipe_cuisine`, `ingredient` → `ingredients`, `category` → `categories` to match the actual product index field names.
3. **Field name normalization in `parseLLMOutput()`**: Added `fieldAliases` map that translates old/wrong field names to correct ones. The LLM might still return `cuisine` despite the prompt saying `recipe_cuisine` — the normalizer catches this. Non-existent fields map to `""` which the validator then rejects.
4. **Updated LLM prompt**: Explicitly tells the LLM not to use `meal_type` or `cooking_method` as filter fields.

**Files changed:** `configs/allowed_filters.yaml`, `configs/llm_prompt.txt`, `internal/infra/bedrock/client.go`

---

## 19. Fuzzy Concept Matcher False Positives on Short Tokens

**Problem:** Searching "chick" returned "ground beef" as a concept. The v3 fuzzy concept recognizer uses `fuzziness: AUTO` which allows 1–2 edit distance. The concept "ground beef" has an alias "ground chuck", and "chick" → "chuck" is only 1 edit (i→u). Since the fuzzy match hit the alias, OpenSearch returned "ground beef" as the top concept match — completely unrelated to what the user was typing.

This class of false positive affects any short token where a single-character change matches an unrelated concept alias buried in a multi-word label.

**Solution:** Added a first-letter guard to the `NativeConceptRecognizer`, mirroring the existing spell correction first-letter guard (problem #17). For fuzzy concept matches (source="fuzzy"), the recognizer now checks that the first letter of the query matches the first letter of at least one word in the concept label:

```go
// "chick" vs "ground beef" → g≠c, b≠c → rejected
// "chick" vs "chicken"     → c=c     → accepted
```

For multi-word labels, the guard checks ANY word (not just the first), since fuzzy matches can hit aliases containing words anywhere in the label.

Exact matches (source="exact") are not affected — the guard only applies to fuzzy hits.

**Files changed:** `internal/domain/native/concept.go`

---

## 20. Comprehension Strips Tokens Too Aggressively

**Problem:** Queries like "quick healthy meals" and "low calorie options" returned 0 results. The comprehension engine was stripping ALL matched tokens from the query — both structural patterns ("under 10") and keyword patterns ("quick", "healthy"). After stripping, the remaining tokens were too few or too generic to match any products in the text search stage.

For example:
- "quick healthy meals" → "quick" stripped (prep_time filter) → "healthy meals" → no concepts, poor text match → 0 results
- "low calorie options" → "low calorie" stripped (calorie filter) → "options" → meaningless search term → 0 results

**Solution:** Selective token stripping based on filter type:

- **Numeric patterns** ("under 10", "500 calories", "less than 20 minutes"): tokens **stripped** — these are structural noise that pollutes text search
- **Keyword patterns** ("quick", "healthy", "easy", "low carb"): tokens **kept** — these are meaningful search terms that improve text relevance
- **Sort patterns** ("cheapest", "newest"): tokens **stripped** — purely structural

The comprehension engine now tracks two separate character maps: `consumedChars` (for overlap prevention between rules) and `stripChars` (for token removal). Only numeric filter and sort matches are marked for stripping.

Filters/sorts are still extracted and applied as `post_filter` by the search orchestrator, so they don't affect text relevance scoring regardless of whether tokens are kept or stripped.

**Files changed:** `internal/domain/pipeline/comprehension.go`, `internal/domain/pipeline/comprehension_test.go`

---

## 21. "Low Calorie Beef" Returns No Beef Results

**Problem:** Searching "low calorie beef" returned 2 results, neither containing beef. The `total_calories` field in the product index is a **keyword** type with string values like `"920 kcal"`, not a numeric field. The comprehension rule `\b(low calories?|low cal|light)\b` extracted a filter `total_calories lte 400`, which performed a lexicographic string comparison — filtering out most products incorrectly. Combined with the `us-diet-low-calorie` concept mapping to the non-existent `dietary` field, "beef" was effectively lost from the search.

Root causes:
1. **Broken numeric filter on keyword field**: `total_calories lte 400` compared strings lexicographically — `"920 kcal" <= "400"` is false (correct by accident), but `"1010 kcal" <= "400"` is true (wrong). Only a handful of random products passed.
2. **Keyword calorie rules consumed overlap region**: The `\b(low calories?|low cal|light)\b` → `total_calories lte 400` rule fired first (calories section before tags section), consuming the character range. The tag-based `Calorie Smart` rule never got a chance to match.
3. **Dietary concepts on non-existent field**: 12 dietary concepts across locales mapped to `field: "dietary"` which doesn't exist in the product index. These resolved in QUS but produced no-op queries in the orchestrator.

**Solution:**
- **Split calorie rules into two** for all 8 languages:
  1. Localized "low calorie" keywords (en: `low calories?|low cal|light`, de: `kalorienarm|leicht`, fr: `peu calorique|léger|allégé`, etc.) → `total_calories lte 400` with `strip: true` — numeric filter that strips matched tokens from text search
  2. `calorie smart|cal smart` → `tags eq "Calorie Smart"` with `strip: true` — exact tag match
- **Added `strip` flag to comprehension engine** (`FilterRule.Strip`, `compiledFilterRule.strip`) — keyword filters with `strip: true` remove their matched tokens from the query, same as numeric filters. Without the flag, "low calorie beef" sent all three words as a single text query, requiring all to match in one field.
- **Kept explicit numeric calorie patterns** (`under 500 calories`, `unter 500 Kalorien`) for when users specify exact numbers
- **Kept dietary concepts** in concept indexes — they serve as semantic metadata even if the orchestrator can't currently query the `dietary` field

Result: "low calorie beef" extracts filter `total_calories lte 400`, strips "low calorie" tokens, and searches remaining token "beef" via text matching. The `calorie smart` tag rule matches only when users type the exact tag name.

**Files changed:** `configs/comprehension.yaml` (all 8 languages), `pkg/config/domain.go`, `internal/domain/pipeline/comprehension.go`

---

## 22. Multi-Word Shingles Claim Too Many Concept Positions

**Problem:** Queries like "spicy asian veggie" and "asian chicken" returned only one concept (e.g. "spicy") instead of matching each token individually. The concept recognizer generates n-gram shingles and searches each against the concept index. A 3-word shingle like "spicy asian veggie" returned partial matches — OS found "spicy" (1-word label) as a hit for the 3-word query. The concept was added with `Start:0, End:2`, claiming all three token positions. The ambiguity resolver then blocked "asian" (pos 1) and "veggie" (pos 2) as overlapping.

The same applied to 2-word shingles: "spicy asian" returned "spicy" and "asian" as individual hits, both assigned span 0-1, blocking later single-token shingles.

**Solution:** Added a word-count guard in the concept recognizer: for multi-word shingles (tokenCount > 1), reject hits where the concept label has fewer words than the shingle. A concept with label "spicy" (1 word) matched from shingle "spicy asian veggie" (3 words) is a partial match — skip it. The single-word concept will be correctly found by its own 1-word shingle with the right position.

This ensures multi-word shingles only produce concepts that genuinely span the full token range (e.g. "chicken tenders" matching a 2-word shingle), while individual tokens are always matched by their own shingles.

**Files changed:** `internal/domain/pipeline/concept.go`
