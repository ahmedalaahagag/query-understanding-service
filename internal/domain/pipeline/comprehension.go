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
// directives from the query using locale-aware regex rules. Matched fragments
// are removed from the token list so they don't pollute the search query.
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

	// Track consumed character ranges in the query string.
	// More specific rules (prep_time, calories) should come before generic price
	// so they consume the region first and prevent the price rule from matching.
	consumedChars := make([]bool, len(query))
	consumedTokens := make(map[int]bool)

	for _, rule := range rules.filterRules {
		loc := rule.re.FindStringIndex(query)
		if loc == nil {
			continue
		}
		// Skip if this match overlaps an already-consumed region.
		if overlapsConsumed(consumedChars, loc[0], loc[1]) {
			continue
		}

		if rule.value != "" {
			// Static value — try numeric first, fall back to string.
			var filterValue interface{} = rule.value
			if v, err := strconv.ParseFloat(rule.value, 64); err == nil {
				filterValue = v * rule.multiplier
			}
			state.Filters = append(state.Filters, model.Filter{
				Field:    rule.field,
				Operator: rule.operator,
				Value:    filterValue,
			})
		} else {
			// Numeric filter — extract captured number.
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
		}

		markConsumedRange(consumedChars, loc[0], loc[1])
		matchedText := strings.ToLower(query[loc[0]:loc[1]])
		markConsumedTokens(state.Tokens, matchedText, consumedTokens)
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
		matchedText := strings.ToLower(query[loc[0]:loc[1]])
		markConsumedTokens(state.Tokens, matchedText, consumedTokens)
		break // Only one sort directive
	}

	// Remove consumed tokens and rebuild the normalized query.
	if len(consumedTokens) > 0 {
		var kept []model.Token
		for i, tok := range state.Tokens {
			if !consumedTokens[i] {
				tok.Position = len(kept)
				kept = append(kept, tok)
			}
		}
		state.Tokens = kept
		rebuildQuery(state)
	}

	return nil
}

// rulesForLocale returns compiled rules for the given locale.
// Tries exact language prefix (e.g. "en" from "en-GB"), then falls back to nil.
func (c *ComprehensionEngine) rulesForLocale(locale string) *compiledLocaleRules {
	if len(c.byLocale) == 0 {
		return nil
	}
	lang := strings.ToLower(locale)
	if idx := strings.IndexAny(lang, "-_"); idx > 0 {
		lang = lang[:idx]
	}
	if rules, ok := c.byLocale[lang]; ok {
		return &rules
	}
	return nil
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

// markConsumedTokens marks token positions whose normalized values appear in the matched text.
func markConsumedTokens(tokens []model.Token, matchedText string, consumed map[int]bool) {
	words := strings.Fields(matchedText)
	wordSet := make(map[string]bool, len(words))
	for _, w := range words {
		wordSet[strings.ToLower(w)] = true
	}
	for i, tok := range tokens {
		if wordSet[strings.ToLower(tok.Normalized)] {
			consumed[i] = true
		}
	}
}

// rebuildQuery is defined in synonym.go (shared across pipeline steps).
