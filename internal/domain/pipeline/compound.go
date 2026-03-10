package pipeline

import (
	"context"
	"strings"

	"github.com/hellofresh/qus/internal/domain/model"
	"github.com/hellofresh/qus/pkg/config"
)

// CompoundHandler is a pipeline step that splits single compound tokens
// and joins multi-token compounds based on configuration rules.
type CompoundHandler struct {
	splits   map[string][2]string // compound → [part1, part2]
	joins    map[string]string    // "part1 part2" → joined
	joinKeys []joinKey            // ordered list for scanning
}

type joinKey struct {
	source []string
	target string
}

// NewCompoundHandler creates a CompoundHandler from configuration.
func NewCompoundHandler(cfg config.CompoundConfig) *CompoundHandler {
	splits := make(map[string][2]string, len(cfg.Split))
	for _, word := range cfg.Split {
		lower := strings.ToLower(word)
		mid := findSplitPoint(lower)
		if mid > 0 {
			splits[lower] = [2]string{lower[:mid], lower[mid:]}
		}
	}

	joins := make(map[string]string, len(cfg.Join))
	var keys []joinKey
	for _, j := range cfg.Join {
		normalized := make([]string, len(j.Source))
		for i, s := range j.Source {
			normalized[i] = strings.ToLower(s)
		}
		key := strings.Join(normalized, " ")
		joins[key] = strings.ToLower(j.Target)
		keys = append(keys, joinKey{source: normalized, target: strings.ToLower(j.Target)})
	}

	return &CompoundHandler{
		splits:   splits,
		joins:    joins,
		joinKeys: keys,
	}
}

func (c *CompoundHandler) Name() string { return "compound" }

func (c *CompoundHandler) Process(_ context.Context, state *model.QueryState) error {
	// Apply joins first (multi-token → single), then splits (single → multi)
	tokens := c.applyJoins(state.Tokens)
	tokens = c.applySplits(tokens)

	if !tokensEqual(state.Tokens, tokens) {
		state.Tokens = tokens
		rebuildQuery(state)
	}

	return nil
}

func (c *CompoundHandler) applyJoins(tokens []model.Token) []model.Token {
	if len(c.joinKeys) == 0 {
		return tokens
	}

	var result []model.Token
	i := 0
	for i < len(tokens) {
		matched := false
		for _, jk := range c.joinKeys {
			srcLen := len(jk.source)
			if i+srcLen > len(tokens) {
				continue
			}

			match := true
			for j, src := range jk.source {
				if tokens[i+j].Normalized != src {
					match = false
					break
				}
			}

			if match {
				result = append(result, model.Token{
					Value:      jk.target,
					Normalized: jk.target,
					Position:   tokens[i].Position,
				})
				i += srcLen
				matched = true
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

func (c *CompoundHandler) applySplits(tokens []model.Token) []model.Token {
	if len(c.splits) == 0 {
		return tokens
	}

	var result []model.Token
	for _, tok := range tokens {
		if parts, ok := c.splits[tok.Normalized]; ok {
			result = append(result,
				model.Token{Value: parts[0], Normalized: parts[0], Position: tok.Position},
				model.Token{Value: parts[1], Normalized: parts[1], Position: tok.Position + 1},
			)
		} else {
			result = append(result, tok)
		}
	}

	return reindex(result)
}

// findSplitPoint uses a simple midpoint heuristic for splitting compound words.
// For production use, this should be config-driven with explicit split points.
func findSplitPoint(word string) int {
	if len(word) < 4 {
		return 0
	}
	return len(word) / 2
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
