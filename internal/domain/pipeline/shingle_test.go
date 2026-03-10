package pipeline

import (
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestGenerateShingles(t *testing.T) {
	tests := []struct {
		name     string
		tokens   []model.Token
		maxSize  int
		expected []model.Shingle
	}{
		{
			name: "three tokens max 2",
			tokens: []model.Token{
				{Value: "cheap", Normalized: "cheap", Position: 0},
				{Value: "chicken", Normalized: "chicken", Position: 1},
				{Value: "burger", Normalized: "burger", Position: 2},
			},
			maxSize: 2,
			expected: []model.Shingle{
				{Text: "cheap chicken", StartPos: 0, EndPos: 1, TokenCount: 2},
				{Text: "chicken burger", StartPos: 1, EndPos: 2, TokenCount: 2},
				{Text: "cheap", StartPos: 0, EndPos: 0, TokenCount: 1},
				{Text: "chicken", StartPos: 1, EndPos: 1, TokenCount: 1},
				{Text: "burger", StartPos: 2, EndPos: 2, TokenCount: 1},
			},
		},
		{
			name: "three tokens max 3",
			tokens: []model.Token{
				{Value: "a", Normalized: "a", Position: 0},
				{Value: "b", Normalized: "b", Position: 1},
				{Value: "c", Normalized: "c", Position: 2},
			},
			maxSize: 3,
			expected: []model.Shingle{
				{Text: "a b c", StartPos: 0, EndPos: 2, TokenCount: 3},
				{Text: "a b", StartPos: 0, EndPos: 1, TokenCount: 2},
				{Text: "b c", StartPos: 1, EndPos: 2, TokenCount: 2},
				{Text: "a", StartPos: 0, EndPos: 0, TokenCount: 1},
				{Text: "b", StartPos: 1, EndPos: 1, TokenCount: 1},
				{Text: "c", StartPos: 2, EndPos: 2, TokenCount: 1},
			},
		},
		{
			name:     "empty tokens",
			tokens:   []model.Token{},
			maxSize:  3,
			expected: nil,
		},
		{
			name: "single token",
			tokens: []model.Token{
				{Value: "burger", Normalized: "burger", Position: 0},
			},
			maxSize: 3,
			expected: []model.Shingle{
				{Text: "burger", StartPos: 0, EndPos: 0, TokenCount: 1},
			},
		},
		{
			name: "max size exceeds token count",
			tokens: []model.Token{
				{Value: "a", Normalized: "a", Position: 0},
				{Value: "b", Normalized: "b", Position: 1},
			},
			maxSize: 5,
			expected: []model.Shingle{
				{Text: "a b", StartPos: 0, EndPos: 1, TokenCount: 2},
				{Text: "a", StartPos: 0, EndPos: 0, TokenCount: 1},
				{Text: "b", StartPos: 1, EndPos: 1, TokenCount: 1},
			},
		},
		{
			name: "zero max size",
			tokens: []model.Token{
				{Value: "a", Normalized: "a", Position: 0},
			},
			maxSize:  0,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateShingles(tt.tokens, tt.maxSize)
			assert.Equal(t, tt.expected, result)
		})
	}
}
