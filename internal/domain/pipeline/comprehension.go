package pipeline

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

type compiledPriceRule struct {
	re         *regexp.Regexp
	field      string
	operator   string
	multiplier float64
}

type compiledSortRule struct {
	re        *regexp.Regexp
	field     string
	direction string
}

// ComprehensionEngine is a pipeline step that extracts price filters and sort
// directives from the query using regex-based rules. Matched fragments are
// removed from the token list so they don't pollute the search query.
type ComprehensionEngine struct {
	priceRules []compiledPriceRule
	sortRules  []compiledSortRule
}

// NewComprehensionEngine creates a ComprehensionEngine from configuration.
func NewComprehensionEngine(cfg config.ComprehensionConfig) *ComprehensionEngine {
	var priceRules []compiledPriceRule
	for _, r := range cfg.PriceRules {
		re, err := regexp.Compile("(?i)" + r.Pattern)
		if err != nil {
			continue
		}
		mult := r.Multiplier
		if mult == 0 {
			mult = 1
		}
		priceRules = append(priceRules, compiledPriceRule{
			re:         re,
			field:      r.Field,
			operator:   r.Operator,
			multiplier: mult,
		})
	}

	var sortRules []compiledSortRule
	for _, r := range cfg.SortRules {
		re, err := regexp.Compile("(?i)" + r.Pattern)
		if err != nil {
			continue
		}
		sortRules = append(sortRules, compiledSortRule{
			re:        re,
			field:     r.Field,
			direction: r.Direction,
		})
	}

	return &ComprehensionEngine{
		priceRules: priceRules,
		sortRules:  sortRules,
	}
}

func (c *ComprehensionEngine) Name() string { return "comprehension" }

func (c *ComprehensionEngine) Process(_ context.Context, state *model.QueryState) error {
	query := state.NormalizedQuery

	// Track which token positions to remove (consumed by filters/sort).
	consumed := make(map[int]bool)

	for _, rule := range c.priceRules {
		loc := rule.re.FindStringIndex(query)
		if loc == nil {
			continue
		}
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
		// Mark tokens covered by this match for removal.
		matchedText := strings.ToLower(query[loc[0]:loc[1]])
		markConsumedTokens(state.Tokens, matchedText, consumed)
	}

	for _, rule := range c.sortRules {
		loc := rule.re.FindStringIndex(query)
		if loc == nil {
			continue
		}
		state.Sort = &model.SortSpec{
			Field:     rule.field,
			Direction: rule.direction,
		}
		matchedText := strings.ToLower(query[loc[0]:loc[1]])
		markConsumedTokens(state.Tokens, matchedText, consumed)
		break // Only one sort directive
	}

	// Remove consumed tokens and rebuild the normalized query.
	if len(consumed) > 0 {
		var kept []model.Token
		for i, tok := range state.Tokens {
			if !consumed[i] {
				tok.Position = len(kept)
				kept = append(kept, tok)
			}
		}
		state.Tokens = kept
		rebuildQuery(state)
	}

	return nil
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
