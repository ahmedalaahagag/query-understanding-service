package hybrid

import (
	"context"

	"github.com/hellofresh/qus/internal/domain/model"
	"github.com/hellofresh/qus/internal/infra/opensearch"
	"github.com/sirupsen/logrus"
)

// ConceptResolver resolves LLM-proposed candidate concepts against OpenSearch.
type ConceptResolver struct {
	searcher opensearch.ConceptSearcher
	logger   *logrus.Logger
}

// NewConceptResolver creates a ConceptResolver that validates concepts via OpenSearch.
func NewConceptResolver(searcher opensearch.ConceptSearcher, logger *logrus.Logger) *ConceptResolver {
	return &ConceptResolver{
		searcher: searcher,
		logger:   logger,
	}
}

// Resolve takes validated LLM candidate concepts and resolves them against the concept index.
// Only concepts that match a known entry in OpenSearch are returned.
func (r *ConceptResolver) Resolve(ctx context.Context, candidates []LLMCandidateConcept, tokens []model.Token, locale, market string) ([]model.ConceptMatch, []string) {
	if r.searcher == nil || len(candidates) == 0 {
		return nil, nil
	}

	var resolved []model.ConceptMatch
	var warnings []string

	for _, candidate := range candidates {
		hits, err := r.searcher.SearchConcepts(ctx, candidate.Label, locale, market)
		if err != nil {
			r.logger.WithError(err).WithField("label", candidate.Label).Warn("concept resolution search failed")
			warnings = append(warnings, "concept resolution failed for: "+candidate.Label)
			continue
		}

		if len(hits) == 0 {
			warnings = append(warnings, "unresolved concept dropped: "+candidate.Label)
			continue
		}

		hit := hits[0]
		start, end := findTokenSpan(tokens, candidate.Label)

		resolved = append(resolved, model.ConceptMatch{
			ID:          hit.ID,
			Label:       hit.Label,
			MatchedText: candidate.Label,
			Field:       hit.Field,
			Score:       candidate.Confidence,
			Source:      "llm",
			Start:       start,
			End:         end,
		})
	}

	return resolved, warnings
}

// findTokenSpan finds the start and end token positions for a label in the token list.
func findTokenSpan(tokens []model.Token, label string) (int, int) {
	for i, tok := range tokens {
		if tok.Normalized == label || tok.Value == label {
			return i, i
		}
	}
	return 0, 0
}
