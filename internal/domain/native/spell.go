package native

import (
	"context"
	"strings"
	"unicode"

	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
)

// NativeSpellCorrector uses OS fuzzy matching against the concept index
// to correct misspelled tokens. Unlike v1's suggest API approach, this
// lets OS handle edit-distance matching natively via fuzziness: AUTO.
type NativeSpellCorrector struct {
	fuzzy  opensearch.FuzzySearcher
	logger *logrus.Logger
}

func NewNativeSpellCorrector(fuzzy opensearch.FuzzySearcher, logger *logrus.Logger) *NativeSpellCorrector {
	return &NativeSpellCorrector{fuzzy: fuzzy, logger: logger}
}

func (n *NativeSpellCorrector) Name() string { return "native_spell" }

func (n *NativeSpellCorrector) Process(ctx context.Context, state *model.QueryState) error {
	if n.fuzzy == nil {
		return nil
	}

	changed := false
	for i, tok := range state.Tokens {
		if shouldSkipToken(tok.Value) {
			continue
		}

		corrected, score, err := n.fuzzy.FuzzySuggest(ctx, tok.Value, state.Locale)
		if err != nil {
			n.logger.WithError(err).WithField("token", tok.Value).Debug("fuzzy suggest failed")
			continue
		}

		if corrected == "" || score == 0 {
			continue
		}

		corrected = strings.ToLower(corrected)
		if corrected == tok.Value {
			continue
		}
		// Only accept corrections within a safe edit distance.
		// Short words (≤5 chars) allow 1 edit; longer words allow 2.
		// Prevents alias matches (e.g. "pasta" → label "ravioli") and
		// valid non-food words from being "corrected" to food terms.
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
		rebuildNormalizedQuery(state)
	}

	return nil
}

func shouldSkipToken(token string) bool {
	if len(token) < 3 {
		return true
	}
	for _, r := range token {
		if !unicode.IsDigit(r) && r != '.' && r != ',' {
			return false
		}
	}
	return true // all digits
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

func rebuildNormalizedQuery(state *model.QueryState) {
	parts := make([]string, len(state.Tokens))
	for i, tok := range state.Tokens {
		parts[i] = tok.Normalized
	}
	state.NormalizedQuery = strings.Join(parts, " ")
}
