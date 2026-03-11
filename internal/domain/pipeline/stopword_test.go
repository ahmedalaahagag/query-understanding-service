package pipeline

import (
	"context"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStopwordFilter_RemovesStopwords(t *testing.T) {
	sw := map[string]bool{"for": true, "with": true, "and": true}
	filter := NewStopwordFilter(sw)

	state := &model.QueryState{
		NormalizedQuery: "chicken for dinner with rice",
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
	sw := map[string]bool{"the": true}
	filter := NewStopwordFilter(sw)

	state := &model.QueryState{
		NormalizedQuery: "the chicken",
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
	sw := map[string]bool{"the": true, "a": true}
	filter := NewStopwordFilter(sw)

	state := &model.QueryState{
		NormalizedQuery: "chicken rice",
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
	sw := map[string]bool{"for": true, "something": true}
	filter := NewStopwordFilter(sw)

	state := &model.QueryState{
		NormalizedQuery: "for something",
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
