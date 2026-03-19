package adaptive

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/native"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type v4GoldenTestCase struct {
	Name     string               `json:"name"`
	Input    v4GoldenInput        `json:"input"`
	Expected v4GoldenExpected     `json:"expected"`
}

type v4GoldenInput struct {
	Query  string `json:"query"`
	Locale string `json:"locale"`
	Market string `json:"market"`
}

type v4GoldenExpected struct {
	model.AnalyzeResponse
	UsedV2     bool `json:"usedV2"`
	V2Fallback bool `json:"v2Fallback"`
}

type v4MockFuzzySearcher struct {
	suggestFn func(ctx context.Context, token, locale string) (string, float64, error)
	searchFn  func(ctx context.Context, text, locale, market string) ([]opensearch.ConceptHit, error)
}

func (m *v4MockFuzzySearcher) FuzzySuggest(ctx context.Context, token, locale string) (string, float64, error) {
	if m.suggestFn != nil {
		return m.suggestFn(ctx, token, locale)
	}
	return "", 0, nil
}

func (m *v4MockFuzzySearcher) FuzzySearchConcepts(ctx context.Context, text, locale, market string) ([]opensearch.ConceptHit, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, text, locale, market)
	}
	return nil, nil
}

func newV4GoldenPipeline() *Pipeline {
	fuzzy := &v4MockFuzzySearcher{
		suggestFn: func(_ context.Context, token, _ string) (string, float64, error) {
			corrections := map[string]string{
				"chiken": "chicken",
				"pizzza": "pizza",
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
			}
			if hits, ok := concepts[text]; ok {
				return hits, nil
			}
			return nil, nil
		},
	}

	stopwords := map[string]map[string]bool{
		"en_gb": {"the": true, "a": true, "an": true, "for": true, "with": true, "and": true, "of": true, "me": true, "something": true, "show": true},
	}

	comprehensionCfg := config.ComprehensionConfig{
		"en": {
			FilterRules: []config.FilterRule{
				{Pattern: `(under|less than)\s+(\d+(?:\.\d+)?)`, Field: "price", Operator: "lt", Multiplier: 100},
			},
			SortRules: []config.SortRule{
				{Pattern: `(cheapest|lowest price)`, Field: "price", Direction: "asc"},
			},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	v3 := native.NewPipeline(native.PipelineConfig{
		FuzzySearcher: fuzzy,
		Comprehension: comprehensionCfg,
		Concept:       config.ConceptConfig{ShingleMaxSize: 3, MaxMatchesPerSpan: 3},
		Stopwords:     stopwords,
		Logger:        logger,
	})

	return NewPipeline(PipelineConfig{
		V3:                      v3,
		V2:                      nil, // no LLM — all queries route to v3
		DirectLLMTokenThreshold: 3,
		Stopwords:               stopwords,
		Logger:                  logger,
	})
}

func TestV4Golden(t *testing.T) {
	pattern := filepath.Join("..", "..", "..", "testdata", "golden", "v4_*.json")
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, files, "no v4 golden test files found matching %s", pattern)

	p := newV4GoldenPipeline()

	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)

		var tc v4GoldenTestCase
		require.NoError(t, json.Unmarshal(data, &tc))

		t.Run(tc.Name, func(t *testing.T) {
			result := p.Run(context.Background(), model.AnalyzeRequest{
				Query:  tc.Input.Query,
				Locale: tc.Input.Locale,
				Market: tc.Input.Market,
			})

			resp := result.Response
			assert.Equal(t, tc.Expected.OriginalQuery, resp.OriginalQuery)
			assert.Equal(t, tc.Expected.NormalizedQuery, resp.NormalizedQuery)
			assert.Equal(t, tc.Expected.Tokens, resp.Tokens)
			assert.Equal(t, tc.Expected.Rewrites, resp.Rewrites)
			assert.Equal(t, tc.Expected.Concepts, resp.Concepts)
			assert.Equal(t, tc.Expected.Filters, resp.Filters)
			assert.Equal(t, tc.Expected.Sort, resp.Sort)
			assert.Equal(t, tc.Expected.UsedV2, result.UsedV2)
			assert.Equal(t, tc.Expected.V2Fallback, result.V2Fallback)
		})
	}
}
