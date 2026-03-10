package pipeline

import (
	"context"
	"testing"

	"github.com/hellofresh/qus/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase",
			input:    "Cheap CHICKEN Burger",
			expected: "cheap chicken burger",
		},
		{
			name:     "trim whitespace",
			input:    "  chicken burger  ",
			expected: "chicken burger",
		},
		{
			name:     "collapse repeated spaces",
			input:    "chicken    burger   meal",
			expected: "chicken burger meal",
		},
		{
			name:     "strip noise punctuation",
			input:    "chicken!! burger??? (meal)",
			expected: "chicken burger meal",
		},
		{
			name:     "preserve hyphens",
			input:    "sugar-free cola",
			expected: "sugar-free cola",
		},
		{
			name:     "preserve apostrophes",
			input:    "it's a meal",
			expected: "it's a meal",
		},
		{
			name:     "strip diacritics",
			input:    "café crème brûlée",
			expected: "cafe creme brulee",
		},
		{
			name:     "unicode normalization",
			input:    "über frühstück",
			expected: "uber fruhstuck",
		},
		{
			name:     "mixed case with punctuation and spaces",
			input:    "  CHEAP!!!   Chiken   BURGER.  ",
			expected: "cheap chiken burger",
		},
		{
			name:     "tabs and newlines collapsed",
			input:    "chicken\t\tburger\n meal",
			expected: "chicken burger meal",
		},
		{
			name:     "empty after normalization",
			input:    "!!!",
			expected: "",
		},
		{
			name:     "already normalized",
			input:    "chicken burger",
			expected: "chicken burger",
		},
		{
			name:     "numbers preserved",
			input:    "under 20 euros",
			expected: "under 20 euros",
		},
		{
			name:     "mixed punctuation",
			input:    "chicken & burger @ home",
			expected: "chicken burger home",
		},
	}

	n := Normalizer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &model.QueryState{
				OriginalQuery:   tt.input,
				NormalizedQuery: tt.input,
			}
			err := n.Process(context.Background(), state)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, state.NormalizedQuery)
		})
	}
}
