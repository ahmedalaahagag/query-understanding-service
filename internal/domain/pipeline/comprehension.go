package pipeline

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

type compiledFilterRule struct {
	re         *regexp.Regexp
	field      string
	operator   string
	value      string  // static value for keyword filters (empty = numeric capture)
	multiplier float64
	strip      bool // strip matched tokens even for keyword filters
}

type compiledSortRule struct {
	re        *regexp.Regexp
	field     string
	direction string
}

type compiledLocaleRules struct {
	filterRules []compiledFilterRule
	sortRules   []compiledSortRule
}

// ComprehensionEngine is a pipeline step that extracts filters and sort
// directives from the query using locale-aware regex rules.
//
// Numeric filter patterns (e.g. "under 10", "less than 500 calories") are
// stripped from tokens because they are structural noise for text search.
// Keyword filter patterns (e.g. "quick", "healthy", "easy") are kept as
// tokens because they are meaningful search terms.
type ComprehensionEngine struct {
	byLocale map[string]compiledLocaleRules
}

// NewComprehensionEngine creates a ComprehensionEngine from configuration.
func NewComprehensionEngine(cfg config.ComprehensionConfig) *ComprehensionEngine {
	byLocale := make(map[string]compiledLocaleRules, len(cfg))

	for lang, rules := range cfg {
		var compiled compiledLocaleRules

		for _, r := range rules.FilterRules {
			re, err := regexp.Compile("(?i)" + r.Pattern)
			if err != nil {
				continue
			}
			mult := r.Multiplier
			if mult == 0 {
				mult = 1
			}
			compiled.filterRules = append(compiled.filterRules, compiledFilterRule{
				re:         re,
				field:      r.Field,
				operator:   r.Operator,
				value:      r.Value,
				multiplier: mult,
				strip:      r.Strip,
			})
		}

		for _, r := range rules.SortRules {
			re, err := regexp.Compile("(?i)" + r.Pattern)
			if err != nil {
				continue
			}
			compiled.sortRules = append(compiled.sortRules, compiledSortRule{
				re:        re,
				field:     r.Field,
				direction: r.Direction,
			})
		}

		byLocale[lang] = compiled
	}

	return &ComprehensionEngine{byLocale: byLocale}
}

func (c *ComprehensionEngine) Name() string { return "comprehension" }

func (c *ComprehensionEngine) Process(_ context.Context, state *model.QueryState) error {
	rules := c.rulesForLocale(state.Locale)
	if rules == nil {
		return nil
	}

	query := state.NormalizedQuery

	// Track consumed character ranges so overlapping rules don't double-match.
	consumedChars := make([]bool, len(query))

	// stripChars tracks ranges to remove from tokens/query.
	// Numeric patterns ("under 10") are stripped; keyword patterns ("quick") are kept.
	stripChars := make([]bool, len(query))

	for _, rule := range rules.filterRules {
		loc := rule.re.FindStringIndex(query)
		if loc == nil {
			continue
		}
		if overlapsConsumed(consumedChars, loc[0], loc[1]) {
			continue
		}

		if rule.value != "" {
			// Keyword filter — keep tokens unless strip is set.
			var filterValue interface{} = rule.value
			if v, err := strconv.ParseFloat(rule.value, 64); err == nil {
				filterValue = v * rule.multiplier
			}
			state.Filters = append(state.Filters, model.Filter{
				Field:    rule.field,
				Operator: rule.operator,
				Value:    filterValue,
			})
			if rule.strip {
				markConsumedRange(stripChars, loc[0], loc[1])
			}
		} else {
			// Numeric filter — strip tokens (structural noise).
			matches := rule.re.FindStringSubmatch(query)
			if len(matches) < 3 {
				continue
			}
			val, err := strconv.ParseFloat(matches[2], 64)
			if err != nil {
				continue
			}
			state.Filters = append(state.Filters, model.Filter{
				Field:    rule.field,
				Operator: rule.operator,
				Value:    val * rule.multiplier,
			})
			markConsumedRange(stripChars, loc[0], loc[1])
		}

		markConsumedRange(consumedChars, loc[0], loc[1])
	}

	for _, rule := range rules.sortRules {
		loc := rule.re.FindStringIndex(query)
		if loc == nil {
			continue
		}
		if overlapsConsumed(consumedChars, loc[0], loc[1]) {
			continue
		}
		state.Sort = &model.SortSpec{
			Field:     rule.field,
			Direction: rule.direction,
		}
		markConsumedRange(consumedChars, loc[0], loc[1])
		markConsumedRange(stripChars, loc[0], loc[1])
		break // Only one sort directive
	}

	// Strip tokens covered by numeric/sort patterns.
	if len(state.Tokens) > 0 {
		consumed := markConsumedTokens(state.Tokens, query, stripChars)
		state.Tokens = removeConsumedTokens(state.Tokens, consumed)
		rebuildQuery(state)
	}

	return nil
}

// rulesForLocale returns compiled rules for the given locale.
// Tries locale-specific key first (e.g. "en_us" from "en-US"), then merges
// with the base language rules (e.g. "en"). Locale-specific rules come first
// so they win via overlapsConsumed when patterns overlap.
func (c *ComprehensionEngine) rulesForLocale(locale string) *compiledLocaleRules {
	if len(c.byLocale) == 0 {
		return nil
	}

	// Normalize: "en-US" → "en_us"
	normalized := strings.ToLower(strings.ReplaceAll(locale, "-", "_"))

	// Extract language prefix: "en_us" → "en"
	lang := normalized
	if idx := strings.Index(lang, "_"); idx > 0 {
		lang = lang[:idx]
	}

	localeRules, hasLocale := c.byLocale[normalized]
	langRules, hasLang := c.byLocale[lang]

	if !hasLocale && !hasLang {
		return nil
	}
	if hasLocale && !hasLang {
		return &localeRules
	}
	if !hasLocale && hasLang {
		return &langRules
	}

	// Merge: locale-specific rules first, then base language rules.
	merged := compiledLocaleRules{
		filterRules: make([]compiledFilterRule, 0, len(localeRules.filterRules)+len(langRules.filterRules)),
		sortRules:   make([]compiledSortRule, 0, len(localeRules.sortRules)+len(langRules.sortRules)),
	}
	merged.filterRules = append(merged.filterRules, localeRules.filterRules...)
	merged.filterRules = append(merged.filterRules, langRules.filterRules...)
	merged.sortRules = append(merged.sortRules, localeRules.sortRules...)
	merged.sortRules = append(merged.sortRules, langRules.sortRules...)
	return &merged
}

// overlapsConsumed checks if a match region overlaps any already-consumed characters.
func overlapsConsumed(consumed []bool, start, end int) bool {
	for i := start; i < end && i < len(consumed); i++ {
		if consumed[i] {
			return true
		}
	}
	return false
}

// markConsumedRange marks a character range as consumed.
func markConsumedRange(consumed []bool, start, end int) {
	for i := start; i < end && i < len(consumed); i++ {
		consumed[i] = true
	}
}

// markConsumedTokens checks which tokens fall within stripped character ranges.
func markConsumedTokens(tokens []model.Token, query string, stripChars []bool) map[int]bool {
	consumed := make(map[int]bool)
	pos := 0
	for i, tok := range tokens {
		idx := strings.Index(query[pos:], tok.Normalized)
		if idx < 0 {
			continue
		}
		start := pos + idx
		end := start + len(tok.Normalized)
		if allConsumed(stripChars, start, end) {
			consumed[i] = true
		}
		pos = end
	}
	return consumed
}

// allConsumed returns true if every character in [start, end) is marked.
func allConsumed(chars []bool, start, end int) bool {
	for i := start; i < end && i < len(chars); i++ {
		if !chars[i] {
			return false
		}
	}
	return true
}

// removeConsumedTokens returns tokens not in the consumed set, with positions renumbered.
func removeConsumedTokens(tokens []model.Token, consumed map[int]bool) []model.Token {
	if len(consumed) == 0 {
		return tokens
	}
	result := make([]model.Token, 0, len(tokens)-len(consumed))
	pos := 0
	for i, tok := range tokens {
		if !consumed[i] {
			tok.Position = pos
			result = append(result, tok)
			pos++
		}
	}
	return result
}
