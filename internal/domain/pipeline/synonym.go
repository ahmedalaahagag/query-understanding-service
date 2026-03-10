package pipeline

import (
	"context"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
)

// SynonymExpander is a pipeline step that expands tokens using linguistic
// dictionary lookups (SYN/HYP) from OpenSearch.
type SynonymExpander struct {
	lookup opensearch.LinguisticLookup
	logger *logrus.Logger
}

// NewSynonymExpander creates a SynonymExpander with an OpenSearch linguistic lookup.
func NewSynonymExpander(lookup opensearch.LinguisticLookup, logger *logrus.Logger) *SynonymExpander {
	return &SynonymExpander{
		lookup: lookup,
		logger: logger,
	}
}

func (s *SynonymExpander) Name() string { return "synonym" }

func (s *SynonymExpander) Process(ctx context.Context, state *model.QueryState) error {
	if s.lookup == nil {
		return nil
	}

	changed := false

	for i, tok := range state.Tokens {
		matches, err := s.lookup.Lookup(ctx, tok.Normalized, state.Locale)
		if err != nil {
			s.logger.WithError(err).WithField("token", tok.Normalized).Warn("linguistic lookup failed")
			continue
		}
		if len(matches) == 0 {
			continue
		}

		best := matches[0]
		// Skip replacement if the input is already a canonical form.
		// This prevents bidirectional synonyms (burger↔hamburger) from
		// replacing one valid term with another.
		if best.IsCanonical {
			continue
		}
		canonical := strings.ToLower(best.Term)
		if canonical != tok.Normalized {
			state.Tokens[i].Normalized = canonical
			changed = true
		}
	}

	if changed {
		rebuildQuery(state)
	}

	return nil
}

func rebuildQuery(state *model.QueryState) {
	parts := make([]string, len(state.Tokens))
	for i, tok := range state.Tokens {
		parts[i] = tok.Normalized
	}
	state.NormalizedQuery = strings.Join(parts, " ")
}
