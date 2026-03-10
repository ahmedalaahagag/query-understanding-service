package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/hellofresh/qus/internal/domain/model"
	"github.com/hellofresh/qus/internal/infra/opensearch"
	"github.com/hellofresh/qus/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConceptSearcher struct {
	results map[string][]opensearch.ConceptHit
	err     error
}

func (m *mockConceptSearcher) SearchConcepts(_ context.Context, text, locale, market string) ([]opensearch.ConceptHit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results[text], nil
}

func defaultConceptConfig() config.ConceptConfig {
	return config.ConceptConfig{
		ShingleMaxSize:    4,
		MaxMatchesPerSpan: 3,
	}
}

func TestConceptRecognizer_ExactMatch(t *testing.T) {
	searcher := &mockConceptSearcher{
		results: map[string][]opensearch.ConceptHit{
			"burger": {
				{ID: "concept_burger", Label: "burger", Field: "product_type", Weight: 10, Score: 5.0, Source: "exact"},
			},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewConceptRecognizer(searcher, defaultConceptConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "cheap chicken burger",
		Tokens: []model.Token{
			{Value: "cheap", Normalized: "cheap", Position: 0},
			{Value: "chicken", Normalized: "chicken", Position: 1},
			{Value: "burger", Normalized: "burger", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	var burgerMatch *model.ConceptMatch
	for _, c := range state.Concepts {
		if c.ID == "concept_burger" {
			burgerMatch = &c
			break
		}
	}
	require.NotNil(t, burgerMatch)
	assert.Equal(t, "burger", burgerMatch.MatchedText)
	assert.Equal(t, "exact", burgerMatch.Source)
	assert.Equal(t, 1.0, burgerMatch.Score)
	assert.Equal(t, 2, burgerMatch.Start)
	assert.Equal(t, 2, burgerMatch.End)
}

func TestConceptRecognizer_MultiTokenShingle(t *testing.T) {
	searcher := &mockConceptSearcher{
		results: map[string][]opensearch.ConceptHit{
			"chicken burger": {
				{ID: "concept_chicken_burger", Label: "chicken burger", Field: "product_type", Weight: 15, Score: 8.0, Source: "exact"},
			},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewConceptRecognizer(searcher, defaultConceptConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "cheap chicken burger",
		Tokens: []model.Token{
			{Value: "cheap", Normalized: "cheap", Position: 0},
			{Value: "chicken", Normalized: "chicken", Position: 1},
			{Value: "burger", Normalized: "burger", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	var found *model.ConceptMatch
	for _, c := range state.Concepts {
		if c.ID == "concept_chicken_burger" {
			found = &c
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, "chicken burger", found.MatchedText)
	assert.Equal(t, 1, found.Start)
	assert.Equal(t, 2, found.End)
}

func TestConceptRecognizer_AliasMatch(t *testing.T) {
	searcher := &mockConceptSearcher{
		results: map[string][]opensearch.ConceptHit{
			"patty": {
				{ID: "concept_burger", Label: "burger", Field: "product_type", Weight: 10, Score: 3.0, Source: "alias"},
			},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewConceptRecognizer(searcher, defaultConceptConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "patty",
		Tokens: []model.Token{
			{Value: "patty", Normalized: "patty", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Concepts, 1)
	assert.Equal(t, "concept_burger", state.Concepts[0].ID)
	assert.Equal(t, "alias", state.Concepts[0].Source)
	assert.Equal(t, 0.7, state.Concepts[0].Score) // alias → default scoreSpell
}

func TestConceptRecognizer_MaxMatchesPerSpan(t *testing.T) {
	searcher := &mockConceptSearcher{
		results: map[string][]opensearch.ConceptHit{
			"burger": {
				{ID: "c1", Label: "burger", Source: "exact"},
				{ID: "c2", Label: "burger", Source: "exact"},
				{ID: "c3", Label: "burger", Source: "exact"},
				{ID: "c4", Label: "burger", Source: "exact"},
			},
		},
	}

	cfg := config.ConceptConfig{ShingleMaxSize: 2, MaxMatchesPerSpan: 2}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewConceptRecognizer(searcher, cfg, logger)

	state := &model.QueryState{
		NormalizedQuery: "burger",
		Tokens: []model.Token{
			{Value: "burger", Normalized: "burger", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// Only max 2 per span
	assert.Len(t, state.Concepts, 2)
}

func TestConceptRecognizer_NilSearcher(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewConceptRecognizer(nil, defaultConceptConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "burger",
		Tokens: []model.Token{
			{Value: "burger", Normalized: "burger", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)
	assert.Empty(t, state.Concepts)
}

func TestConceptRecognizer_SearchError(t *testing.T) {
	searcher := &mockConceptSearcher{
		err: errors.New("opensearch down"),
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewConceptRecognizer(searcher, defaultConceptConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "burger",
		Tokens: []model.Token{
			{Value: "burger", Normalized: "burger", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)
	assert.Empty(t, state.Concepts)
}

func TestConceptRecognizer_EmptyTokens(t *testing.T) {
	searcher := &mockConceptSearcher{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewConceptRecognizer(searcher, defaultConceptConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "",
		Tokens:          []model.Token{},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)
	assert.Empty(t, state.Concepts)
}
