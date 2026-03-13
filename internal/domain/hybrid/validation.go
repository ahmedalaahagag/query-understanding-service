package hybrid

import (
	"fmt"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
)

// ValidatedIntent holds the LLM output after validation — only safe fields survive.
type ValidatedIntent struct {
	NormalizedQuery   string
	Rewrites          []string
	CandidateConcepts []LLMCandidateConcept
	Filters           []LLMFilter
	Sort              *LLMSort
	Warnings          []string
}

// Validator checks LLM output against configured allowlists and thresholds.
type Validator struct {
	allowedFilters map[string]config.AllowedFilter
	allowedSorts   map[string]bool
	minConfidence  float64
}

// NewValidator creates a Validator from config.
func NewValidator(filters config.AllowedFiltersConfig, sorts config.AllowedSortsConfig, minConfidence float64) *Validator {
	fm := make(map[string]config.AllowedFilter, len(filters.Filters))
	for _, f := range filters.Filters {
		fm[f.Field] = f
	}

	sm := make(map[string]bool, len(sorts.Sorts))
	for _, s := range sorts.Sorts {
		sm[s.Key] = true
	}

	return &Validator{
		allowedFilters: fm,
		allowedSorts:   sm,
		minConfidence:  minConfidence,
	}
}

// Validate checks LLM output and returns only valid, allowed fields.
// The query parameter is used to verify that inferred filter values actually
// appear in the user's input (prevents false inference like "burger" → cuisine "American").
func (v *Validator) Validate(result *LLMParseResult, query string) ValidatedIntent {
	normalizedQuery := validateNormalizedQuery(result.NormalizedQuery, query, 2)

	intent := ValidatedIntent{
		NormalizedQuery: normalizedQuery.query,
		Rewrites:        result.Rewrites,
	}
	intent.Warnings = append(intent.Warnings, normalizedQuery.warnings...)

	if result.Confidence < v.minConfidence {
		intent.Warnings = append(intent.Warnings, fmt.Sprintf(
			"overall confidence %.2f below threshold %.2f, dropping semantic output",
			result.Confidence, v.minConfidence,
		))
		return intent
	}

	intent.Filters = v.validateFilters(result.Filters, query, &intent.Warnings)
	intent.Sort = v.validateSort(result.Sort, &intent.Warnings)
	intent.CandidateConcepts = v.validateConcepts(result.CandidateConcepts, &intent.Warnings)
	intent.Warnings = append(intent.Warnings, result.Warnings...)

	return intent
}

type normalizedQueryResult struct {
	query    string
	warnings []string
}

// validateNormalizedQuery checks the LLM's normalized query against the original.
// When both have the same word count, it compares positionally — each changed word
// must be a plausible typo correction (within edit distance). When word counts
// differ (LLM added/removed words), it falls back to set-based checking.
// If any word is a hallucinated substitution, the entire normalized query is reverted.
func validateNormalizedQuery(llmQuery, originalQuery string, maxEdits int) normalizedQueryResult {
	if llmQuery == "" || strings.EqualFold(llmQuery, originalQuery) {
		return normalizedQueryResult{query: llmQuery}
	}

	origWords := strings.Fields(strings.ToLower(originalQuery))
	llmWords := strings.Fields(strings.ToLower(llmQuery))

	// Same word count: compare positionally to catch substitutions like "party"→"pasta"
	// even when the replacement word exists elsewhere in the query.
	if len(origWords) == len(llmWords) {
		for i, llmW := range llmWords {
			origW := origWords[i]
			if llmW == origW {
				continue
			}
			wordMax := maxEdits
			if len(origW) <= 5 {
				wordMax = min(1, maxEdits)
			}
			if levenshtein(llmW, origW) > wordMax {
				return normalizedQueryResult{
					query: strings.ToLower(originalQuery),
					warnings: []string{fmt.Sprintf(
						"LLM replaced %q with %q (edit distance %d > %d), reverting normalizedQuery",
						origW, llmW, levenshtein(llmW, origW), wordMax,
					)},
				}
			}
		}
		return normalizedQueryResult{query: llmQuery}
	}

	// Different word counts: fall back to set-based check.
	origSet := make(map[string]bool, len(origWords))
	for _, w := range origWords {
		origSet[w] = true
	}

	for _, w := range llmWords {
		if origSet[w] {
			continue
		}
		wordMax := maxEdits
		if len(w) <= 5 {
			wordMax = min(1, maxEdits)
		}
		if closestEditDistance(w, origWords) <= wordMax {
			continue
		}
		return normalizedQueryResult{
			query: strings.ToLower(originalQuery),
			warnings: []string{fmt.Sprintf(
				"LLM introduced word %q not found in original query, reverting normalizedQuery",
				w,
			)},
		}
	}

	return normalizedQueryResult{query: llmQuery}
}

// closestEditDistance returns the minimum Levenshtein distance between word and
// any word in candidates.
func closestEditDistance(word string, candidates []string) int {
	best := len(word) + 1
	for _, c := range candidates {
		d := levenshtein(word, c)
		if d < best {
			best = d
		}
	}
	return best
}

// levenshtein computes the Levenshtein edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func (v *Validator) validateFilters(filters []LLMFilter, query string, warnings *[]string) []LLMFilter {
	queryLower := strings.ToLower(query)

	var valid []LLMFilter
	for _, f := range filters {
		allowed, ok := v.allowedFilters[f.Field]
		if !ok {
			*warnings = append(*warnings, fmt.Sprintf("filter field %q not allowed, dropped", f.Field))
			continue
		}

		if !containsString(allowed.Operators, f.Operator) {
			*warnings = append(*warnings, fmt.Sprintf("operator %q not allowed for field %q, dropped", f.Operator, f.Field))
			continue
		}

		if f.Confidence < v.minConfidence {
			*warnings = append(*warnings, fmt.Sprintf("filter %s %s confidence %.2f below threshold, dropped", f.Field, f.Operator, f.Confidence))
			continue
		}

		// Ground-truth check: the raw LLM value must trace back to something
		// the user actually wrote. This prevents all false inference — not just
		// keyword types (e.g. "burger" → cuisine "American", or an invented
		// numeric threshold the user never mentioned).
		if !isValueGrounded(f.Value, queryLower, allowed.WordValues) {
			*warnings = append(*warnings, fmt.Sprintf(
				"filter %s value %v not found in query, dropped (possible false inference)",
				f.Field, f.Value))
			continue
		}

		// Coerce non-numeric values for number-typed fields.
		if allowed.Type == "number" {
			if _, isNum := f.Value.(float64); !isNum {
				// Try to resolve word values (e.g. "cheap" → 8).
				if str, ok := f.Value.(string); ok && len(allowed.WordValues) > 0 {
					if num, found := allowed.WordValues[strings.ToLower(str)]; found {
						f.Value = num
					} else {
						*warnings = append(*warnings, fmt.Sprintf("filter %s value %q is not a number, dropped", f.Field, str))
						continue
					}
				} else {
					*warnings = append(*warnings, fmt.Sprintf("filter %s value %v is not a number, dropped", f.Field, f.Value))
					continue
				}
			}
			// Apply multiplier (e.g. price in cents: user says 20, index stores 2000).
			if allowed.Multiplier > 0 {
				if num, ok := f.Value.(float64); ok {
					f.Value = num * allowed.Multiplier
				}
			}
		}

		valid = append(valid, f)
	}
	return valid
}

func (v *Validator) validateSort(sort *LLMSort, warnings *[]string) *LLMSort {
	if sort == nil {
		return nil
	}

	// Try composite key: field + "_" + direction (e.g. "price" + "asc" -> "price_asc")
	sortKey := sort.Field + "_" + sort.Direction
	if sort.Field == "relevance" {
		sortKey = "relevance"
	}

	// Also check if the model already returned a composite key as field (e.g. "price_asc")
	if !v.allowedSorts[sortKey] && v.allowedSorts[sort.Field] {
		sortKey = sort.Field
	}

	if !v.allowedSorts[sortKey] {
		*warnings = append(*warnings, fmt.Sprintf("sort %q not allowed, dropped", sortKey))
		return nil
	}

	if sort.Confidence < v.minConfidence {
		*warnings = append(*warnings, fmt.Sprintf("sort %s confidence %.2f below threshold, dropped", sortKey, sort.Confidence))
		return nil
	}

	return sort
}

func (v *Validator) validateConcepts(concepts []LLMCandidateConcept, warnings *[]string) []LLMCandidateConcept {
	var valid []LLMCandidateConcept
	for _, c := range concepts {
		if c.Confidence < v.minConfidence {
			*warnings = append(*warnings, fmt.Sprintf("concept %q confidence %.2f below threshold, dropped", c.Label, c.Confidence))
			continue
		}
		valid = append(valid, c)
	}
	return valid
}

// isValueGrounded checks whether a filter value can be traced back to the
// user's query. This prevents the LLM from hallucinating filter values
// that the user never mentioned (e.g. "burger" → cuisine "American").
//
// For string values: the string must appear in the query (case-insensitive).
// For numeric values: the number (formatted as an integer or float) must appear.
// Word-value aliases (e.g. "cheap" mapping to a number) are also accepted.
func isValueGrounded(value interface{}, queryLower string, wordValues map[string]float64) bool {
	switch v := value.(type) {
	case string:
		if strings.Contains(queryLower, strings.ToLower(v)) {
			return true
		}
		// Check if it's a recognized word-value alias present in the query.
		if _, ok := wordValues[strings.ToLower(v)]; ok {
			return strings.Contains(queryLower, strings.ToLower(v))
		}
		return false
	case float64:
		// Check integer form first (20 vs 20.00).
		if v == float64(int(v)) {
			if strings.Contains(queryLower, fmt.Sprintf("%d", int(v))) {
				return true
			}
		} else if strings.Contains(queryLower, fmt.Sprintf("%g", v)) {
			return true
		}
		// Check if the number maps back to a word-value alias in the query.
		// E.g. LLM returns 1 for difficulty_level, and "easy" → 1 is a
		// word_value — accept if "easy" appears in the query.
		for word, num := range wordValues {
			if num == v && strings.Contains(queryLower, strings.ToLower(word)) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
