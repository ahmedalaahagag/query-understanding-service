package hybrid

import (
	"fmt"
	"os"
	"strings"

	"github.com/hellofresh/qus/pkg/config"
)

// PromptBuilder constructs the system prompt and user message for the LLM.
type PromptBuilder struct {
	systemPromptTemplate string
	filters              config.AllowedFiltersConfig
	sorts                config.AllowedSortsConfig
}

// NewPromptBuilder loads the system prompt template and allowed configs.
func NewPromptBuilder(promptFile string, filters config.AllowedFiltersConfig, sorts config.AllowedSortsConfig) (*PromptBuilder, error) {
	data, err := os.ReadFile(promptFile)
	if err != nil {
		return nil, fmt.Errorf("reading prompt file: %w", err)
	}
	return &PromptBuilder{
		systemPromptTemplate: string(data),
		filters:              filters,
		sorts:                sorts,
	}, nil
}

// SystemPrompt returns the full system prompt with allowed filters and sorts injected.
func (p *PromptBuilder) SystemPrompt() string {
	var b strings.Builder
	b.WriteString(p.systemPromptTemplate)
	b.WriteString("\nFILTERS: ")
	for i, f := range p.filters.Filters {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(fmt.Sprintf("%s(%s)%s", f.Field, strings.Join(f.Operators, ","), f.Type))
	}
	b.WriteString("\nSORTS: ")
	keys := make([]string, 0, len(p.sorts.Sorts))
	for _, s := range p.sorts.Sorts {
		keys = append(keys, s.Key)
	}
	b.WriteString(strings.Join(keys, ","))
	return b.String()
}

// UserMessage formats the user query with locale and market context.
func (p *PromptBuilder) UserMessage(query, locale, market string) string {
	return fmt.Sprintf("Query: %s\nLocale: %s\nMarket: %s", query, locale, market)
}
