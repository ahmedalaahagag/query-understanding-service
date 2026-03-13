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

type mockSpellChecker struct {
	suggestions map[string][]opensearch.SpellSuggestion
	err         error
}

func (m *mockSpellChecker) Suggest(_ context.Context, text, _ string) ([]opensearch.SpellSuggestion, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.suggestions[text], nil
}

func defaultSpellConfig() config.SpellConfig {
	return config.SpellConfig{
		Enabled:             true,
		MinTokenLength:      4,
		ConfidenceThreshold: 0.85,
	}
}

func TestSpellResolver_CorrectsMisspelling(t *testing.T) {
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"chiken": {{Text: "chicken", Score: 0.95}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		OriginalQuery:   "cheap chiken burger",
		NormalizedQuery: "cheap chiken burger",
		Tokens: []model.Token{
			{Value: "cheap", Normalized: "cheap", Position: 0},
			{Value: "chiken", Normalized: "chiken", Position: 1},
			{Value: "burger", Normalized: "burger", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chicken", state.Tokens[1].Normalized)
	assert.Equal(t, "cheap chicken burger", state.NormalizedQuery)
}

func TestSpellResolver_SkipsShortTokens(t *testing.T) {
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"the": {{Text: "thee", Score: 0.95}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "the burger",
		Tokens: []model.Token{
			{Value: "the", Normalized: "the", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "the", state.Tokens[0].Normalized)
	assert.Equal(t, "the burger", state.NormalizedQuery)
}

func TestSpellResolver_SkipsNumericTokens(t *testing.T) {
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"12345": {{Text: "12346", Score: 0.99}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "12345 burger",
		Tokens: []model.Token{
			{Value: "12345", Normalized: "12345", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "12345", state.Tokens[0].Normalized)
}

func TestSpellResolver_SkipsSKUPatterns(t *testing.T) {
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"AB1234": {{Text: "AB1235", Score: 0.99}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "AB1234",
		Tokens: []model.Token{
			{Value: "AB1234", Normalized: "AB1234", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "AB1234", state.Tokens[0].Normalized)
}

func TestSpellResolver_BelowThreshold(t *testing.T) {
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"borger": {{Text: "burger", Score: 0.60}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "borger",
		Tokens: []model.Token{
			{Value: "borger", Normalized: "borger", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "borger", state.Tokens[0].Normalized)
	assert.Equal(t, "borger", state.NormalizedQuery)
}

func TestSpellResolver_NoSuggestions(t *testing.T) {
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

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

func TestSpellResolver_ErrorFallsBackGracefully(t *testing.T) {
	checker := &mockSpellChecker{
		err: errors.New("opensearch unavailable"),
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "chiken burger",
		Tokens: []model.Token{
			{Value: "chiken", Normalized: "chiken", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// Keeps original tokens when OpenSearch fails
	assert.Equal(t, "chiken", state.Tokens[0].Normalized)
	assert.Equal(t, "chiken burger", state.NormalizedQuery)
	assert.Contains(t, state.Warnings, "spell check failed for token: chiken")
}

func TestSpellResolver_Disabled(t *testing.T) {
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"chiken": {{Text: "chicken", Score: 0.95}},
		},
	}

	cfg := defaultSpellConfig()
	cfg.Enabled = false

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, cfg, logger)

	state := &model.QueryState{
		NormalizedQuery: "chiken",
		Tokens: []model.Token{
			{Value: "chiken", Normalized: "chiken", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chiken", state.Tokens[0].Normalized)
}

func TestSpellResolver_NilChecker(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(nil, defaultSpellConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "chiken",
		Tokens: []model.Token{
			{Value: "chiken", Normalized: "chiken", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chiken", state.Tokens[0].Normalized)
}

func TestSpellResolver_MultipleCorrections(t *testing.T) {
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"chiken": {{Text: "chicken", Score: 0.95}},
			"burgar": {{Text: "burger", Score: 0.92}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		OriginalQuery:   "cheap chiken burgar",
		NormalizedQuery: "cheap chiken burgar",
		Tokens: []model.Token{
			{Value: "cheap", Normalized: "cheap", Position: 0},
			{Value: "chiken", Normalized: "chiken", Position: 1},
			{Value: "burgar", Normalized: "burgar", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chicken", state.Tokens[1].Normalized)
	assert.Equal(t, "burger", state.Tokens[2].Normalized)
	assert.Equal(t, "cheap chicken burger", state.NormalizedQuery)
}

func TestSpellResolver_RejectsValidWordCorrection(t *testing.T) {
	// "cheese" → "chinese" is 2 edits on a 6-char word. The spell checker
	// should reject this: if the suggest API returns nothing (suggest_mode=missing
	// means no suggestion when the term exists in the index), the token is kept.
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			// No suggestions for "cheese" — it exists in the index
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		OriginalQuery:   "mac and cheese",
		NormalizedQuery: "mac cheese",
		Tokens: []model.Token{
			{Value: "mac", Normalized: "mac", Position: 0},
			{Value: "cheese", Normalized: "cheese", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "cheese", state.Tokens[1].Normalized)
	assert.Equal(t, "mac cheese", state.NormalizedQuery)
}

func TestSpellResolver_RejectsFirstLetterChange(t *testing.T) {
	// "dinner" → "ginger" is 2 edits on a 6-char word (allowed by Levenshtein),
	// but changes the first letter — almost never a valid correction.
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"dinner": {{Text: "ginger", Score: 0.90}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		NormalizedQuery: "dinner",
		Tokens: []model.Token{
			{Value: "dinner", Normalized: "dinner", Position: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "dinner", state.Tokens[0].Normalized, "should reject correction that changes first letter")
}

func TestSpellResolver_RejectsTwoEditOnShortWord(t *testing.T) {
	// "party" → "pasta" is 2 edits on a 5-char word. The spell checker should
	// reject this because short words (≤5 chars) only allow 1 edit.
	checker := &mockSpellChecker{
		suggestions: map[string][]opensearch.SpellSuggestion{
			"party": {{Text: "pasta", Score: 0.8}},
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	step := NewSpellResolver(checker, defaultSpellConfig(), logger)

	state := &model.QueryState{
		OriginalQuery:   "sunday party",
		NormalizedQuery: "sunday party",
		Tokens: []model.Token{
			{Value: "sunday", Normalized: "sunday", Position: 0},
			{Value: "party", Normalized: "party", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "party", state.Tokens[1].Normalized)
	assert.Equal(t, "sunday party", state.NormalizedQuery)
}
