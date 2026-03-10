package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLinguisticLookup struct {
	results map[string][]opensearch.LinguisticMatch
	err     error
}

func (m *mockLinguisticLookup) Lookup(_ context.Context, term, locale string) ([]opensearch.LinguisticMatch, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results[term], nil
}

func testSynonymConfig() config.SynonymConfig {
	return config.SynonymConfig{
		Locale: "en-GB",
		Entries: []config.SynonymEntry{
			{Canonical: "vegetarian", Variants: []string{"veggie", "veg"}},
			{Canonical: "coca cola", Variants: []string{"coke"}},
		},
	}
}

func TestSynonymExpander_FallbackConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(nil, testSynonymConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "veggie burger",
		Tokens: []model.Token{
			{Value: "veggie", Normalized: "veggie", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "vegetarian", state.Tokens[0].Normalized)
	assert.Equal(t, "vegetarian burger", state.NormalizedQuery)
}

func TestSynonymExpander_LinguisticLookup(t *testing.T) {
	lookup := &mockLinguisticLookup{
		results: map[string][]opensearch.LinguisticMatch{
			"sneakers": {{Term: "trainers", Type: "SYN"}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, config.SynonymConfig{Locale: "en-GB"}, logger)

	state := &model.QueryState{
		NormalizedQuery: "sneakers",
		Tokens: []model.Token{
			{Value: "sneakers", Normalized: "sneakers", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "trainers", state.Tokens[0].Normalized)
	assert.Equal(t, "trainers", state.NormalizedQuery)
}

func TestSynonymExpander_LinguisticFallsBackToConfig(t *testing.T) {
	lookup := &mockLinguisticLookup{
		err: errors.New("opensearch down"),
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, testSynonymConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "veggie burger",
		Tokens: []model.Token{
			{Value: "veggie", Normalized: "veggie", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "vegetarian", state.Tokens[0].Normalized)
	assert.Equal(t, "vegetarian burger", state.NormalizedQuery)
}

func TestSynonymExpander_LinguisticPrioritizedOverConfig(t *testing.T) {
	// Linguistic says veggie → plant-based, config says veggie → vegetarian
	lookup := &mockLinguisticLookup{
		results: map[string][]opensearch.LinguisticMatch{
			"veggie": {{Term: "plant-based", Type: "SYN"}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, testSynonymConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "veggie",
		Tokens: []model.Token{
			{Value: "veggie", Normalized: "veggie", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// Linguistic index takes priority
	assert.Equal(t, "plant-based", state.Tokens[0].Normalized)
}

func TestSynonymExpander_NoMatch(t *testing.T) {
	lookup := &mockLinguisticLookup{
		results: map[string][]opensearch.LinguisticMatch{},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, config.SynonymConfig{Locale: "en-GB"}, logger)

	state := &model.QueryState{
		NormalizedQuery: "chicken burger",
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chicken burger", state.NormalizedQuery)
}

func TestSynonymExpander_CaseInsensitive(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(nil, testSynonymConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "veg meal",
		Tokens: []model.Token{
			{Value: "veg", Normalized: "veg", Position: 0},
			{Value: "meal", Normalized: "meal", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "vegetarian", state.Tokens[0].Normalized)
}
