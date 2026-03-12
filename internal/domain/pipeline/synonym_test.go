package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
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

func TestSynonymExpander_LinguisticLookup(t *testing.T) {
	lookup := &mockLinguisticLookup{
		results: map[string][]opensearch.LinguisticMatch{
			"sneakers": {{Term: "trainers", Type: "SYN"}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, logger)

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

func TestSynonymExpander_LookupError(t *testing.T) {
	lookup := &mockLinguisticLookup{
		err: errors.New("opensearch down"),
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, logger)

	state := &model.QueryState{
		NormalizedQuery: "veggie burger",
		Tokens: []model.Token{
			{Value: "veggie", Normalized: "veggie", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// No changes on error
	assert.Equal(t, "veggie burger", state.NormalizedQuery)
}

func TestSynonymExpander_NoMatch(t *testing.T) {
	lookup := &mockLinguisticLookup{
		results: map[string][]opensearch.LinguisticMatch{},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, logger)

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

func TestSynonymExpander_SkipsCanonicalForm(t *testing.T) {
	lookup := &mockLinguisticLookup{
		results: map[string][]opensearch.LinguisticMatch{
			"burger": {{Term: "hamburger", Type: "SYN", IsCanonical: true}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, logger)

	state := &model.QueryState{
		NormalizedQuery: "burger",
		Tokens: []model.Token{
			{Value: "burger", Normalized: "burger", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "burger", state.Tokens[0].Normalized)
}

func TestSynonymExpander_SkipsHypernymReplacement(t *testing.T) {
	lookup := &mockLinguisticLookup{
		results: map[string][]opensearch.LinguisticMatch{
			"pasta": {{Term: "ravioli", Type: "HYP"}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(lookup, logger)

	state := &model.QueryState{
		NormalizedQuery: "pasta",
		Tokens: []model.Token{
			{Value: "pasta", Normalized: "pasta", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// HYP should NOT replace — "pasta" must stay "pasta", not become "ravioli".
	assert.Equal(t, "pasta", state.Tokens[0].Normalized)
	assert.Equal(t, "pasta", state.NormalizedQuery)
}

func TestSynonymExpander_NilLookup(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSynonymExpander(nil, logger)

	state := &model.QueryState{
		NormalizedQuery: "veggie burger",
		Tokens: []model.Token{
			{Value: "veggie", Normalized: "veggie", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "veggie burger", state.NormalizedQuery)
}
