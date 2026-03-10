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

type mockCompoundLookup struct {
	entries map[string][]opensearch.CompoundEntry
	err     error
}

func (m *mockCompoundLookup) LookupCompounds(_ context.Context, text, _ string) ([]opensearch.CompoundEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.entries[text], nil
}

func testCompoundLookup() *mockCompoundLookup {
	return &mockCompoundLookup{
		entries: map[string][]opensearch.CompoundEntry{
			// Join rules: "ice cream" → "icecream", "peanut butter" → "peanutbutter"
			"ice cream":     {{Compound: "icecream", Parts: "ice cream"}},
			"peanut butter": {{Compound: "peanutbutter", Parts: "peanut butter"}},
			// Split rules: "crewneck" → "crew neck", "lunchbox" → "lunch box"
			"crewneck": {{Compound: "crewneck", Parts: "crew neck"}},
			"lunchbox": {{Compound: "lunchbox", Parts: "lunch box"}},
		},
	}
}

func newTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.ErrorLevel)
	return l
}

func TestCompoundHandler_Join(t *testing.T) {
	step := NewCompoundHandler(testCompoundLookup(), newTestLogger())

	state := &model.QueryState{
		NormalizedQuery: "ice cream cake",
		Tokens: []model.Token{
			{Value: "ice", Normalized: "ice", Position: 0},
			{Value: "cream", Normalized: "cream", Position: 1},
			{Value: "cake", Normalized: "cake", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Tokens, 2)
	assert.Equal(t, "icecream", state.Tokens[0].Normalized)
	assert.Equal(t, 0, state.Tokens[0].Position)
	assert.Equal(t, "cake", state.Tokens[1].Normalized)
	assert.Equal(t, 1, state.Tokens[1].Position)
	assert.Equal(t, "icecream cake", state.NormalizedQuery)
}

func TestCompoundHandler_Split(t *testing.T) {
	step := NewCompoundHandler(testCompoundLookup(), newTestLogger())

	state := &model.QueryState{
		NormalizedQuery: "crewneck sweater",
		Tokens: []model.Token{
			{Value: "crewneck", Normalized: "crewneck", Position: 0},
			{Value: "sweater", Normalized: "sweater", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Tokens, 3)
	assert.Equal(t, "crew", state.Tokens[0].Normalized)
	assert.Equal(t, 0, state.Tokens[0].Position)
	assert.Equal(t, "neck", state.Tokens[1].Normalized)
	assert.Equal(t, 1, state.Tokens[1].Position)
	assert.Equal(t, "sweater", state.Tokens[2].Normalized)
	assert.Equal(t, 2, state.Tokens[2].Position)
	assert.Equal(t, "crew neck sweater", state.NormalizedQuery)
}

func TestCompoundHandler_JoinAndSplit(t *testing.T) {
	step := NewCompoundHandler(testCompoundLookup(), newTestLogger())

	state := &model.QueryState{
		NormalizedQuery: "ice cream lunchbox",
		Tokens: []model.Token{
			{Value: "ice", Normalized: "ice", Position: 0},
			{Value: "cream", Normalized: "cream", Position: 1},
			{Value: "lunchbox", Normalized: "lunchbox", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Tokens, 3)
	assert.Equal(t, "icecream", state.Tokens[0].Normalized)
	assert.Equal(t, "lunch", state.Tokens[1].Normalized)
	assert.Equal(t, "box", state.Tokens[2].Normalized)
}

func TestCompoundHandler_NoMatch(t *testing.T) {
	step := NewCompoundHandler(testCompoundLookup(), newTestLogger())

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
	assert.Len(t, state.Tokens, 2)
}

func TestCompoundHandler_NilLookup(t *testing.T) {
	step := NewCompoundHandler(nil, newTestLogger())

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

func TestCompoundHandler_MultipleJoins(t *testing.T) {
	step := NewCompoundHandler(testCompoundLookup(), newTestLogger())

	state := &model.QueryState{
		NormalizedQuery: "peanut butter ice cream",
		Tokens: []model.Token{
			{Value: "peanut", Normalized: "peanut", Position: 0},
			{Value: "butter", Normalized: "butter", Position: 1},
			{Value: "ice", Normalized: "ice", Position: 2},
			{Value: "cream", Normalized: "cream", Position: 3},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Tokens, 2)
	assert.Equal(t, "peanutbutter", state.Tokens[0].Normalized)
	assert.Equal(t, "icecream", state.Tokens[1].Normalized)
	assert.Equal(t, "peanutbutter icecream", state.NormalizedQuery)
}

func TestCompoundHandler_LookupError(t *testing.T) {
	lookup := &mockCompoundLookup{err: errors.New("opensearch down")}
	step := NewCompoundHandler(lookup, newTestLogger())

	state := &model.QueryState{
		NormalizedQuery: "crewneck sweater",
		Tokens: []model.Token{
			{Value: "crewneck", Normalized: "crewneck", Position: 0},
			{Value: "sweater", Normalized: "sweater", Position: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// No changes on error — tokens preserved
	assert.Equal(t, "crewneck sweater", state.NormalizedQuery)
}
