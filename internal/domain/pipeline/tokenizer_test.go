package pipeline

import (
	"context"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenizer(t *testing.T) {
	tests := []struct {
		name           string
		normalizedQuery string
		expectedTokens []model.Token
	}{
		{
			name:           "simple split",
			normalizedQuery: "cheap chicken burger",
			expectedTokens: []model.Token{
				{Value: "cheap", Normalized: "cheap", Position: 0},
				{Value: "chicken", Normalized: "chicken", Position: 1},
				{Value: "burger", Normalized: "burger", Position: 2},
			},
		},
		{
			name:           "single token",
			normalizedQuery: "burger",
			expectedTokens: []model.Token{
				{Value: "burger", Normalized: "burger", Position: 0},
			},
		},
		{
			name:           "empty query",
			normalizedQuery: "",
			expectedTokens: []model.Token{},
		},
		{
			name:           "hyphenated word preserved",
			normalizedQuery: "sugar-free cola",
			expectedTokens: []model.Token{
				{Value: "sugar-free", Normalized: "sugar-free", Position: 0},
				{Value: "cola", Normalized: "cola", Position: 1},
			},
		},
		{
			name:           "many tokens with positions",
			normalizedQuery: "a b c d e",
			expectedTokens: []model.Token{
				{Value: "a", Normalized: "a", Position: 0},
				{Value: "b", Normalized: "b", Position: 1},
				{Value: "c", Normalized: "c", Position: 2},
				{Value: "d", Normalized: "d", Position: 3},
				{Value: "e", Normalized: "e", Position: 4},
			},
		},
		{
			name:           "numeric tokens",
			normalizedQuery: "under 20",
			expectedTokens: []model.Token{
				{Value: "under", Normalized: "under", Position: 0},
				{Value: "20", Normalized: "20", Position: 1},
			},
		},
	}

	tok := Tokenizer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &model.QueryState{
				NormalizedQuery: tt.normalizedQuery,
			}
			err := tok.Process(context.Background(), state)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTokens, state.Tokens)
		})
	}
}
