package pipeline

import (
	"context"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

// StopwordFilter removes stopword tokens before concept recognition so that
// generic words like "for", "with", "something" don't cause false positives
// in concept search.
type StopwordFilter struct {
	stopwords map[string]bool
}

// NewStopwordFilter creates a StopwordFilter with the given stopword set.
// If stopwords is nil or empty, the step is a no-op.
func NewStopwordFilter(stopwords map[string]bool) *StopwordFilter {
	return &StopwordFilter{stopwords: stopwords}
}

func (s *StopwordFilter) Name() string { return "stopword" }

func (s *StopwordFilter) Process(_ context.Context, state *model.QueryState) error {
	if len(s.stopwords) == 0 || len(state.Tokens) == 0 {
		return nil
	}

	var kept []model.Token
	for _, t := range state.Tokens {
		if s.stopwords[strings.ToLower(t.Normalized)] {
			continue
		}
		t.Position = len(kept)
		kept = append(kept, t)
	}

	if len(kept) < len(state.Tokens) {
		state.Tokens = kept
		rebuildQuery(state)
	}

	return nil
}
