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
		if corrected != tok.Value {
			state.Tokens[i].Normalized = corrected
			changed = true
		}
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

func rebuildNormalizedQuery(state *model.QueryState) {
	parts := make([]string, len(state.Tokens))
	for i, tok := range state.Tokens {
		parts[i] = tok.Normalized
	}
	state.NormalizedQuery = strings.Join(parts, " ")
}
