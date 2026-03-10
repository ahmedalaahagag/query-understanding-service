package pipeline

import (
	"context"
	"strings"

	"github.com/hellofresh/qus/internal/domain/model"
)

// Tokenizer is a pipeline step that splits the normalized query on whitespace
// and assigns positional indices to each token.
type Tokenizer struct{}

func (Tokenizer) Name() string { return "tokenize" }

func (Tokenizer) Process(_ context.Context, state *model.QueryState) error {
	parts := strings.Fields(state.NormalizedQuery)
	tokens := make([]model.Token, 0, len(parts))

	for i, part := range parts {
		tokens = append(tokens, model.Token{
			Value:      part,
			Normalized: part,
			Position:   i,
		})
	}

	state.Tokens = tokens
	return nil
}
