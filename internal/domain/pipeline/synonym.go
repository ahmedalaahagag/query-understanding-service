package pipeline

import (
	"context"
	"strings"

	"github.com/hellofresh/qus/pkg/model"
	"github.com/hellofresh/qus/internal/infra/opensearch"
	"github.com/hellofresh/qus/pkg/config"
	"github.com/sirupsen/logrus"
)

// SynonymExpander is a pipeline step that expands tokens using linguistic
// dictionary lookups (SYN/HYP) and falls back to YAML config synonyms.
// Expansions carry boost weights for downstream ranking differentiation.
type SynonymExpander struct {
	lookup   opensearch.LinguisticLookup
	fallback map[string]string // variant → canonical from YAML config
	logger   *logrus.Logger
}

// NewSynonymExpander creates a SynonymExpander with an OpenSearch linguistic lookup
// and a YAML-based fallback synonym map.
func NewSynonymExpander(lookup opensearch.LinguisticLookup, synCfg config.SynonymConfig, logger *logrus.Logger) *SynonymExpander {
	fallback := buildFallbackMap(synCfg)
	return &SynonymExpander{
		lookup:   lookup,
		fallback: fallback,
		logger:   logger,
	}
}

func (s *SynonymExpander) Name() string { return "synonym" }

func (s *SynonymExpander) Process(ctx context.Context, state *model.QueryState) error {
	changed := false

	for i, tok := range state.Tokens {
		// Try OpenSearch linguistic index first
		if s.lookup != nil {
			matches, err := s.lookup.Lookup(ctx, tok.Normalized, state.Locale)
			if err != nil {
				s.logger.WithError(err).WithField("token", tok.Normalized).Warn("linguistic lookup failed, trying fallback")
			} else if len(matches) > 0 {
				best := matches[0]
				canonical := strings.ToLower(best.Term)
				if canonical != tok.Normalized {
					state.Tokens[i].Normalized = canonical
					changed = true
					continue
				}
			}
		}

		// Fallback to YAML config
		if canonical, ok := s.fallback[tok.Normalized]; ok {
			state.Tokens[i].Normalized = canonical
			changed = true
		}
	}

	if changed {
		rebuildQuery(state)
	}

	return nil
}

func buildFallbackMap(cfg config.SynonymConfig) map[string]string {
	m := make(map[string]string)
	for _, entry := range cfg.Entries {
		canonical := strings.ToLower(entry.Canonical)
		for _, variant := range entry.Variants {
			m[strings.ToLower(variant)] = canonical
		}
	}
	return m
}

func rebuildQuery(state *model.QueryState) {
	parts := make([]string, len(state.Tokens))
	for i, tok := range state.Tokens {
		parts[i] = tok.Normalized
	}
	state.NormalizedQuery = strings.Join(parts, " ")
}
