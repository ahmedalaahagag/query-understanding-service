package pipeline

import (
	"context"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stopwords(locale string, words ...string) map[string]map[string]bool {
	sw := make(map[string]bool, len(words))
	for _, w := range words {
		sw[w] = true
	}
	return map[string]map[string]bool{locale: sw}
}

func TestStopwordFilter_RemovesStopwords(t *testing.T) {
	filter := NewStopwordFilter(stopwords("en_gb", "for", "with", "and"))

	state := &model.QueryState{
		NormalizedQuery: "chicken for dinner with rice",
		Locale:          "en-GB",
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
			{Value: "for", Normalized: "for", Position: 1},
			{Value: "dinner", Normalized: "dinner", Position: 2},
			{Value: "with", Normalized: "with", Position: 3},
			{Value: "rice", Normalized: "rice", Position: 4},
		},
	}

	err := filter.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chicken dinner rice", state.NormalizedQuery)
	assert.Len(t, state.Tokens, 3)
	assert.Equal(t, "chicken", state.Tokens[0].Normalized)
	assert.Equal(t, 0, state.Tokens[0].Position)
	assert.Equal(t, "dinner", state.Tokens[1].Normalized)
	assert.Equal(t, 1, state.Tokens[1].Position)
	assert.Equal(t, "rice", state.Tokens[2].Normalized)
	assert.Equal(t, 2, state.Tokens[2].Position)
}

func TestStopwordFilter_CaseInsensitive(t *testing.T) {
	filter := NewStopwordFilter(stopwords("en_gb", "the"))

	state := &model.QueryState{
		NormalizedQuery: "the chicken",
		Locale:          "en-GB",
		Tokens: []model.Token{
			{Value: "The", Normalized: "the", Position: 0},
			{Value: "chicken", Normalized: "chicken", Position: 1},
		},
	}

	err := filter.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chicken", state.NormalizedQuery)
	assert.Len(t, state.Tokens, 1)
}

func TestStopwordFilter_NoStopwords(t *testing.T) {
	filter := NewStopwordFilter(nil)

	state := &model.QueryState{
		NormalizedQuery: "chicken rice",
		Locale:          "en-GB",
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
			{Value: "rice", Normalized: "rice", Position: 1},
		},
	}

	err := filter.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chicken rice", state.NormalizedQuery)
	assert.Len(t, state.Tokens, 2)
}

func TestStopwordFilter_NoMatch(t *testing.T) {
	filter := NewStopwordFilter(stopwords("en_gb", "the", "a"))

	state := &model.QueryState{
		NormalizedQuery: "chicken rice",
		Locale:          "en-GB",
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
			{Value: "rice", Normalized: "rice", Position: 1},
		},
	}

	err := filter.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "chicken rice", state.NormalizedQuery)
	assert.Len(t, state.Tokens, 2)
}

func TestStopwordFilter_AllStopwords(t *testing.T) {
	filter := NewStopwordFilter(stopwords("en_gb", "for", "something"))

	state := &model.QueryState{
		NormalizedQuery: "for something",
		Locale:          "en-GB",
		Tokens: []model.Token{
			{Value: "for", Normalized: "for", Position: 0},
			{Value: "something", Normalized: "something", Position: 1},
		},
	}

	err := filter.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "", state.NormalizedQuery)
	assert.Empty(t, state.Tokens)
}

func TestStopwordFilter_DifferentLocale(t *testing.T) {
	byLocale := map[string]map[string]bool{
		"en_gb": {"the": true, "for": true},
		"de_de": {"der": true, "die": true, "das": true},
	}
	filter := NewStopwordFilter(byLocale)

	// German query should use German stopwords
	state := &model.QueryState{
		NormalizedQuery: "das hähnchen",
		Locale:          "de-DE",
		Tokens: []model.Token{
			{Value: "das", Normalized: "das", Position: 0},
			{Value: "hähnchen", Normalized: "hähnchen", Position: 1},
		},
	}

	err := filter.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Equal(t, "hähnchen", state.NormalizedQuery)
	assert.Len(t, state.Tokens, 1)
}

func TestStopwordFilter_UnknownLocale(t *testing.T) {
	filter := NewStopwordFilter(stopwords("en_gb", "the"))

	state := &model.QueryState{
		NormalizedQuery: "the chicken",
		Locale:          "xx-XX",
		Tokens: []model.Token{
			{Value: "the", Normalized: "the", Position: 0},
			{Value: "chicken", Normalized: "chicken", Position: 1},
		},
	}

	err := filter.Process(context.Background(), state)
	require.NoError(t, err)

	// No stopwords for unknown locale — tokens unchanged
	assert.Equal(t, "the chicken", state.NormalizedQuery)
	assert.Len(t, state.Tokens, 2)
}
