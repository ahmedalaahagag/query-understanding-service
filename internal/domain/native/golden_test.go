package native

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type v3GoldenTestCase struct {
	Name     string               `json:"name"`
	Input    v3GoldenInput        `json:"input"`
	Expected model.AnalyzeResponse `json:"expected"`
}

type v3GoldenInput struct {
	Query  string `json:"query"`
	Locale string `json:"locale"`
	Market string `json:"market"`
}

func newV3GoldenPipeline() *Pipeline {
	fuzzy := &mockFuzzySearcher{
		suggestFn: func(_ context.Context, token, _ string) (string, float64, error) {
			corrections := map[string]string{
				"chiken":  "chicken",
				"pizzza":  "pizza",
				"tomatoe": "tomato",
			}
			if corrected, ok := corrections[token]; ok {
				return corrected, 5.0, nil
			}
			return "", 0, nil
		},
		searchFn: func(_ context.Context, text, _, _ string) ([]opensearch.ConceptHit, error) {
			concepts := map[string][]opensearch.ConceptHit{
				"chicken": {{ID: "c-chicken", Label: "chicken", Field: "protein", Score: 10.0, Source: "exact"}},
				"pizza":   {{ID: "c-pizza", Label: "pizza", Field: "category", Score: 10.0, Source: "exact"}},
				"pasta":   {{ID: "c-pasta", Label: "pasta", Field: "category", Score: 10.0, Source: "exact"}},
				"tomato":  {{ID: "c-tomato", Label: "tomato", Field: "ingredient", Score: 8.0, Source: "exact"}},
				"soup":    {{ID: "c-soup", Label: "soup", Field: "category", Score: 9.0, Source: "exact"}},
			}
			if hits, ok := concepts[text]; ok {
				return hits, nil
			}
			return nil, nil
		},
	}

	stopwords := map[string]map[string]bool{
		"en_gb": {"the": true, "a": true, "an": true, "for": true, "with": true, "and": true, "of": true},
		"de_de": {"der": true, "die": true, "das": true, "und": true, "mit": true, "für": true},
	}

	comprehensionCfg := config.ComprehensionConfig{
		"en": {
			FilterRules: []config.FilterRule{
				{Pattern: `(under|less than)\s+(\d+)\s*(minutes?|mins?)`, Field: "preparation_time", Operator: "lt", Multiplier: 60},
				{Pattern: `\b(quick|fast)\b`, Field: "preparation_time", Operator: "lte", Value: "1800"},
				{Pattern: `(under|less than|cheaper than)\s+(\d+(?:\.\d+)?)`, Field: "price", Operator: "lt", Multiplier: 100},
				{Pattern: `\b(no gluten|gluten[ -]?free)\b`, Field: "dietary", Operator: "eq", Value: "Gluten-Free Friendly", Strip: true},
				{Pattern: `\b(not spicy|no spicy|no spice)\b`, Field: "dietary", Operator: "eq", Value: "Non-spicy", Strip: true},
			},
			SortRules: []config.SortRule{
				{Pattern: `(cheapest|lowest price)`, Field: "price", Direction: "asc"},
			},
		},
	}

	return NewPipeline(PipelineConfig{
		FuzzySearcher: fuzzy,
		Comprehension: comprehensionCfg,
		Concept:       config.ConceptConfig{ShingleMaxSize: 3, MaxMatchesPerSpan: 3},
		Stopwords:     stopwords,
		Logger:        func() *logrus.Logger { l := logrus.New(); l.SetLevel(logrus.ErrorLevel); return l }(),
	})
}

func TestV3Golden(t *testing.T) {
	pattern := filepath.Join("..", "..", "..", "testdata", "golden", "v3_*.json")
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, files, "no v3 golden test files found matching %s", pattern)

	p := newV3GoldenPipeline()

	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)

		var tc v3GoldenTestCase
		require.NoError(t, json.Unmarshal(data, &tc))

		t.Run(tc.Name, func(t *testing.T) {
			resp, err := p.Run(context.Background(), model.AnalyzeRequest{
				Query:  tc.Input.Query,
				Locale: tc.Input.Locale,
				Market: tc.Input.Market,
			})
			require.NoError(t, err)

			assert.Equal(t, tc.Expected.OriginalQuery, resp.OriginalQuery)
			assert.Equal(t, tc.Expected.NormalizedQuery, resp.NormalizedQuery)
			assert.Equal(t, tc.Expected.Tokens, resp.Tokens)
			assert.Equal(t, tc.Expected.Rewrites, resp.Rewrites)
			assert.Equal(t, tc.Expected.Concepts, resp.Concepts)
			assert.Equal(t, tc.Expected.Filters, resp.Filters)
			assert.Equal(t, tc.Expected.Sort, resp.Sort)
		})
	}
}
