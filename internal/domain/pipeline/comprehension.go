package pipeline

import (
	"context"
	"regexp"
	"strconv"

	"github.com/hellofresh/qus/internal/domain/model"
	"github.com/hellofresh/qus/pkg/config"
)

type compiledPriceRule struct {
	re       *regexp.Regexp
	field    string
	operator string
}

type compiledSortRule struct {
	re        *regexp.Regexp
	field     string
	direction string
}

// ComprehensionEngine is a pipeline step that extracts price filters and sort
// directives from the query using regex-based rules.
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
		priceRules = append(priceRules, compiledPriceRule{
			re:       re,
			field:    r.Field,
			operator: r.Operator,
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

	for _, rule := range c.priceRules {
		matches := rule.re.FindStringSubmatch(query)
		if len(matches) >= 3 {
			val, err := strconv.ParseFloat(matches[2], 64)
			if err != nil {
				continue
			}
			state.Filters = append(state.Filters, model.Filter{
				Field:    rule.field,
				Operator: rule.operator,
				Value:    val,
			})
		}
	}

	for _, rule := range c.sortRules {
		if rule.re.MatchString(query) {
			state.Sort = &model.SortSpec{
				Field:     rule.field,
				Direction: rule.direction,
			}
			break // Only one sort directive
		}
	}

	return nil
}
