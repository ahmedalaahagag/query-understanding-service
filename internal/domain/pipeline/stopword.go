package pipeline

import (
	"context"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

// StopwordFilter removes stopword tokens before concept recognition so that
// generic words like "for", "with", "something" don't cause false positives
// in concept search. Supports per-locale stopword sets.
type StopwordFilter struct {
	byLocale map[string]map[string]bool
}

// NewStopwordFilter creates a StopwordFilter with per-locale stopword sets.
// The map key is the normalized locale (e.g. "en_gb"). If nil or empty, the
// step is a no-op.
func NewStopwordFilter(byLocale map[string]map[string]bool) *StopwordFilter {
	return &StopwordFilter{byLocale: byLocale}
}

func (s *StopwordFilter) Name() string { return "stopword" }

func (s *StopwordFilter) Process(_ context.Context, state *model.QueryState) error {
	if len(s.byLocale) == 0 || len(state.Tokens) == 0 {
		return nil
	}

	locale := normalizeLocale(state.Locale)
	stopwords := s.byLocale[locale]
	if len(stopwords) == 0 {
		return nil
	}

	var kept []model.Token
	for _, t := range state.Tokens {
		if stopwords[strings.ToLower(t.Normalized)] {
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

// normalizeLocale converts "en-GB" to "en_gb" for map lookup.
func normalizeLocale(locale string) string {
	return strings.ToLower(strings.ReplaceAll(locale, "-", "_"))
}
