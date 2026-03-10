package hybrid

import (
	"github.com/hellofresh/qus/pkg/model"
)

// BuildFallbackResponse creates a minimal deterministic response when the LLM path fails.
func BuildFallbackResponse(originalQuery, normalizedQuery string, tokens []model.Token, reason string) model.AnalyzeResponse {
	if tokens == nil {
		tokens = []model.Token{}
	}

	rewrites := []string{}
	if normalizedQuery != originalQuery {
		rewrites = append(rewrites, normalizedQuery)
	}

	return model.AnalyzeResponse{
		OriginalQuery:   originalQuery,
		NormalizedQuery: normalizedQuery,
		Tokens:          tokens,
		Rewrites:        rewrites,
		Concepts:        []model.ConceptMatch{},
		Filters:         []model.Filter{},
		Warnings:        []string{"llm fallback: " + reason},
	}
}
