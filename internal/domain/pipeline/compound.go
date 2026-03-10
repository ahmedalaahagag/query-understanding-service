package pipeline

import (
	"context"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
)

const maxJoinWindow = 3

// CompoundHandler is a pipeline step that splits single compound tokens
// and joins multi-token compounds using OpenSearch linguistic index (type=CMP).
type CompoundHandler struct {
	lookup opensearch.CompoundLookup
	logger *logrus.Logger
}

// NewCompoundHandler creates a CompoundHandler backed by OpenSearch.
func NewCompoundHandler(lookup opensearch.CompoundLookup, logger *logrus.Logger) *CompoundHandler {
	return &CompoundHandler{
		lookup: lookup,
		logger: logger,
	}
}

func (c *CompoundHandler) Name() string { return "compound" }

func (c *CompoundHandler) Process(ctx context.Context, state *model.QueryState) error {
	if c.lookup == nil {
		return nil
	}

	// Apply joins first (multi-token → single), then splits (single → multi)
	tokens := c.applyJoins(ctx, state.Tokens, state.Locale)
	tokens = c.applySplits(ctx, tokens, state.Locale)

	if !tokensEqual(state.Tokens, tokens) {
		state.Tokens = tokens
		rebuildQuery(state)
	}

	return nil
}

func (c *CompoundHandler) applyJoins(ctx context.Context, tokens []model.Token, locale string) []model.Token {
	var result []model.Token
	i := 0
	for i < len(tokens) {
		matched := false
		// Try largest window first (e.g., 3 tokens, then 2)
		for window := maxJoinWindow; window >= 2; window-- {
			if i+window > len(tokens) {
				continue
			}

			parts := make([]string, window)
			for j := 0; j < window; j++ {
				parts[j] = tokens[i+j].Normalized
			}
			text := strings.Join(parts, " ")

			entries, err := c.lookup.LookupCompounds(ctx, text, locale)
			if err != nil {
				c.logger.WithError(err).WithField("text", text).Warn("compound join lookup failed")
				continue
			}

			for _, e := range entries {
				if e.Parts == text {
					result = append(result, model.Token{
						Value:      e.Compound,
						Normalized: e.Compound,
						Position:   tokens[i].Position,
					})
					i += window
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			result = append(result, tokens[i])
			i++
		}
	}

	return reindex(result)
}

func (c *CompoundHandler) applySplits(ctx context.Context, tokens []model.Token, locale string) []model.Token {
	var result []model.Token
	for _, tok := range tokens {
		entries, err := c.lookup.LookupCompounds(ctx, tok.Normalized, locale)
		if err != nil {
			c.logger.WithError(err).WithField("token", tok.Normalized).Warn("compound split lookup failed")
			result = append(result, tok)
			continue
		}

		split := false
		for _, e := range entries {
			if e.Compound == tok.Normalized {
				parts := strings.Fields(e.Parts)
				for _, p := range parts {
					result = append(result, model.Token{
						Value:      p,
						Normalized: p,
						Position:   tok.Position,
					})
				}
				split = true
				break
			}
		}
		if !split {
			result = append(result, tok)
		}
	}

	return reindex(result)
}

func reindex(tokens []model.Token) []model.Token {
	for i := range tokens {
		tokens[i].Position = i
	}
	return tokens
}

func tokensEqual(a, b []model.Token) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Normalized != b[i].Normalized {
			return false
		}
	}
	return true
}
