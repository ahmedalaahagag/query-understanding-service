package hybrid

import (
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

// BuildFallbackResponse creates a deterministic response when the LLM path fails.
// It uses filters/sort extracted by comprehension (if any) so the orchestrator
// still applies price ranges, sort orders, etc.
func BuildFallbackResponse(state *model.QueryState, reason string) model.AnalyzeResponse {
	tokens := state.Tokens
	if tokens == nil {
		tokens = []model.Token{}
	}

	rewrites := []string{}
	if state.NormalizedQuery != state.OriginalQuery {
		rewrites = append(rewrites, state.NormalizedQuery)
	}

	filters := state.Filters
	if filters == nil {
		filters = []model.Filter{}
	}

	return model.AnalyzeResponse{
		OriginalQuery:   state.OriginalQuery,
		NormalizedQuery: state.NormalizedQuery,
		Tokens:          tokens,
		Rewrites:        rewrites,
		Concepts:        []model.ConceptMatch{},
		Filters:         filters,
		Sort:            state.Sort,
		Warnings:        []string{"llm fallback: " + reason},
	}
}
