package pipeline

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/sirupsen/logrus"
)

var skuPattern = regexp.MustCompile(`^[A-Za-z]{0,3}\d{3,}`)

// SpellResolver is a pipeline step that corrects misspelled tokens using OpenSearch suggest.
type SpellResolver struct {
	checker opensearch.SpellChecker
	cfg     config.SpellConfig
	logger  *logrus.Logger
}

// NewSpellResolver creates a new SpellResolver step.
func NewSpellResolver(checker opensearch.SpellChecker, cfg config.SpellConfig, logger *logrus.Logger) *SpellResolver {
	return &SpellResolver{
		checker: checker,
		cfg:     cfg,
		logger:  logger,
	}
}

func (s *SpellResolver) Name() string { return "spell" }

func (s *SpellResolver) Process(ctx context.Context, state *model.QueryState) error {
	if !s.cfg.Enabled || s.checker == nil {
		return nil
	}

	changed := false
	for i, tok := range state.Tokens {
		if s.shouldSkip(tok.Value) {
			continue
		}

		suggestions, err := s.checker.Suggest(ctx, tok.Value, state.Locale)
		if err != nil {
			s.logger.WithError(err).WithField("token", tok.Value).Warn("spell suggest failed, keeping original")
			state.Warnings = append(state.Warnings, "spell check failed for token: "+tok.Value)
			continue
		}

		if len(suggestions) == 0 {
			continue
		}

		best := suggestions[0]
		if best.Score < s.cfg.ConfidenceThreshold {
			continue
		}

		corrected := strings.ToLower(best.Text)
		if corrected == tok.Value {
			continue
		}
		// Only accept corrections within a safe edit distance.
		// Short words (≤5 chars) allow 1 edit; longer words allow 2.
		// This prevents a domain-specific concept index from "correcting"
		// valid non-food words to food terms (e.g. "party" → "pasta").
		maxEdits := 2
		if len(tok.Value) <= 5 {
			maxEdits = 1
		}
		if levenshtein(corrected, tok.Value) > maxEdits {
			continue
		}
		// Reject corrections that change the first letter.
		// Valid typos almost never start with a different letter
		// (e.g. "dinner" → "ginger" is a false correction).
		if corrected[0] != tok.Value[0] {
			continue
		}
		state.Tokens[i].Normalized = corrected
		changed = true
	}

	if changed {
		s.rebuildNormalizedQuery(state)
	}

	return nil
}

func (s *SpellResolver) shouldSkip(token string) bool {
	// Skip short tokens
	if len(token) < s.cfg.MinTokenLength {
		return true
	}

	// Skip numeric tokens
	if isNumeric(token) {
		return true
	}

	// Skip likely SKU patterns (e.g. AB123, 12345)
	if skuPattern.MatchString(token) {
		return true
	}

	return false
}

func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) && r != '.' && r != ',' {
			return false
		}
	}
	return true
}

// levenshtein computes the Levenshtein edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func (s *SpellResolver) rebuildNormalizedQuery(state *model.QueryState) {
	parts := make([]string, len(state.Tokens))
	for i, tok := range state.Tokens {
		parts[i] = tok.Normalized
	}
	state.NormalizedQuery = strings.Join(parts, " ")
}
