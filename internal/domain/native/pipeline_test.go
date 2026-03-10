package native

import (
	"context"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFuzzySearcher struct {
	suggestFn func(ctx context.Context, token, locale string) (string, float64, error)
	searchFn  func(ctx context.Context, text, locale, market string) ([]opensearch.ConceptHit, error)
}

func (m *mockFuzzySearcher) FuzzySuggest(ctx context.Context, token, locale string) (string, float64, error) {
	if m.suggestFn != nil {
		return m.suggestFn(ctx, token, locale)
	}
	return "", 0, nil
}

func (m *mockFuzzySearcher) FuzzySearchConcepts(ctx context.Context, text, locale, market string) ([]opensearch.ConceptHit, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, text, locale, market)
	}
	return nil, nil
}

func TestNativePipeline_FullFlow(t *testing.T) {
	fuzzy := &mockFuzzySearcher{
		suggestFn: func(_ context.Context, token, _ string) (string, float64, error) {
			if token == "chiken" {
				return "chicken", 5.0, nil
			}
			return "", 0, nil
		},
		searchFn: func(_ context.Context, text, _, _ string) ([]opensearch.ConceptHit, error) {
			if text == "chicken" {
				return []opensearch.ConceptHit{
					{ID: "cat-chicken", Label: "chicken", Field: "category", Score: 10.0, Source: "exact"},
				}, nil
			}
			return nil, nil
		},
	}

	p := NewPipeline(PipelineConfig{
		FuzzySearcher: fuzzy,
		Concept:       config.ConceptConfig{ShingleMaxSize: 3, MaxMatchesPerSpan: 3},
		Comprehension: config.ComprehensionConfig{
			PriceRules: []config.PriceRule{
				{Pattern: `(under|less than)\s+(\d+(?:\.\d+)?)`, Field: "price", Operator: "lt"},
			},
		},
		Logger: logrus.New(),
	})

	resp, err := p.Run(context.Background(), model.AnalyzeRequest{
		Query:  "Chiken under 10",
		Locale: "en-GB",
		Market: "uk",
	})

	require.NoError(t, err)
	assert.Equal(t, "Chiken under 10", resp.OriginalQuery)
	assert.Equal(t, "chicken under 10", resp.NormalizedQuery)
	assert.True(t, len(resp.Concepts) > 0, "should find chicken concept")
	assert.Equal(t, "chicken", resp.Concepts[0].Label)
	require.Len(t, resp.Filters, 1)
	assert.Equal(t, "price", resp.Filters[0].Field)
	assert.Equal(t, "lt", resp.Filters[0].Operator)
	assert.Equal(t, 10.0, resp.Filters[0].Value)
}

func TestNativePipeline_NoFuzzyMatch(t *testing.T) {
	fuzzy := &mockFuzzySearcher{}

	p := NewPipeline(PipelineConfig{
		FuzzySearcher: fuzzy,
		Logger:        logrus.New(),
	})

	resp, err := p.Run(context.Background(), model.AnalyzeRequest{
		Query:  "random query",
		Locale: "en-GB",
		Market: "uk",
	})

	require.NoError(t, err)
	assert.Equal(t, "random query", resp.NormalizedQuery)
	assert.Empty(t, resp.Concepts)
}

func TestNativePipeline_SpellCorrectionAndRewrites(t *testing.T) {
	fuzzy := &mockFuzzySearcher{
		suggestFn: func(_ context.Context, token, _ string) (string, float64, error) {
			if token == "vegatable" {
				return "vegetable", 4.5, nil
			}
			return "", 0, nil
		},
	}

	p := NewPipeline(PipelineConfig{
		FuzzySearcher: fuzzy,
		Logger:        logrus.New(),
	})

	resp, err := p.Run(context.Background(), model.AnalyzeRequest{
		Query:  "vegatable soup",
		Locale: "en-GB",
		Market: "uk",
	})

	require.NoError(t, err)
	assert.Equal(t, "vegetable soup", resp.NormalizedQuery)
	assert.Contains(t, resp.Rewrites, "vegetable soup")
}

func TestNativePipeline_SortExtraction(t *testing.T) {
	fuzzy := &mockFuzzySearcher{}

	p := NewPipeline(PipelineConfig{
		FuzzySearcher: fuzzy,
		Comprehension: config.ComprehensionConfig{
			SortRules: []config.SortRule{
				{Pattern: `(cheapest|lowest price)`, Field: "price", Direction: "asc"},
			},
		},
		Logger: logrus.New(),
	})

	resp, err := p.Run(context.Background(), model.AnalyzeRequest{
		Query:  "cheapest meals",
		Locale: "en-GB",
		Market: "uk",
	})

	require.NoError(t, err)
	require.NotNil(t, resp.Sort)
	assert.Equal(t, "price", resp.Sort.Field)
	assert.Equal(t, "asc", resp.Sort.Direction)
}

func TestNativePipeline_NilFuzzySearcher(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		Logger: logrus.New(),
	})

	resp, err := p.Run(context.Background(), model.AnalyzeRequest{
		Query:  "chicken soup",
		Locale: "en-GB",
		Market: "uk",
	})

	require.NoError(t, err)
	assert.Equal(t, "chicken soup", resp.NormalizedQuery)
	assert.Len(t, resp.Tokens, 2)
}

func TestNativeSpellCorrector_SkipsShortTokens(t *testing.T) {
	called := false
	fuzzy := &mockFuzzySearcher{
		suggestFn: func(_ context.Context, _ string, _ string) (string, float64, error) {
			called = true
			return "", 0, nil
		},
	}

	corrector := NewNativeSpellCorrector(fuzzy, logrus.New())
	state := &model.QueryState{
		Tokens: []model.Token{
			{Value: "ab", Normalized: "ab", Position: 0},
		},
	}

	err := corrector.Process(context.Background(), state)
	require.NoError(t, err)
	assert.False(t, called, "should skip short tokens")
}

func TestNativeSpellCorrector_SkipsNumericTokens(t *testing.T) {
	called := false
	fuzzy := &mockFuzzySearcher{
		suggestFn: func(_ context.Context, _ string, _ string) (string, float64, error) {
			called = true
			return "", 0, nil
		},
	}

	corrector := NewNativeSpellCorrector(fuzzy, logrus.New())
	state := &model.QueryState{
		Tokens: []model.Token{
			{Value: "123", Normalized: "123", Position: 0},
		},
	}

	err := corrector.Process(context.Background(), state)
	require.NoError(t, err)
	assert.False(t, called, "should skip numeric tokens")
}

func TestNativeConceptRecognizer_FuzzyMatch(t *testing.T) {
	fuzzy := &mockFuzzySearcher{
		searchFn: func(_ context.Context, text, _, _ string) ([]opensearch.ConceptHit, error) {
			if text == "chickn" {
				return []opensearch.ConceptHit{
					{ID: "cat-chicken", Label: "chicken", Field: "category", Score: 8.5, Source: "fuzzy"},
				}, nil
			}
			return nil, nil
		},
	}

	recognizer := NewNativeConceptRecognizer(fuzzy, config.ConceptConfig{ShingleMaxSize: 3, MaxMatchesPerSpan: 3}, logrus.New())
	state := &model.QueryState{
		Tokens: []model.Token{
			{Value: "chickn", Normalized: "chickn", Position: 0},
		},
		Locale: "en-GB",
		Market: "uk",
	}

	err := recognizer.Process(context.Background(), state)
	require.NoError(t, err)
	require.Len(t, state.Concepts, 1)
	assert.Equal(t, "chicken", state.Concepts[0].Label)
	assert.Equal(t, "fuzzy", state.Concepts[0].Source)
}
