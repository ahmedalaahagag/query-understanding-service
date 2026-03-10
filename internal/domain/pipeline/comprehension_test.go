package pipeline

import (
	"context"
	"testing"

	"github.com/hellofresh/qus/pkg/model"
	"github.com/hellofresh/qus/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testComprehensionConfig() config.ComprehensionConfig {
	return config.ComprehensionConfig{
		PriceRules: []config.PriceRule{
			{Pattern: `(under|less than|cheaper than)\s+(\d+(?:\.\d+)?)`, Field: "price", Operator: "lt"},
		},
		SortRules: []config.SortRule{
			{Pattern: `(cheapest|lowest price)`, Field: "price", Direction: "asc"},
			{Pattern: `(newest|most recent)`, Field: "created_at", Direction: "desc"},
		},
	}
}

func TestComprehension_PriceUnder(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())

	state := &model.QueryState{
		NormalizedQuery: "chicken burger under 20",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "price", state.Filters[0].Field)
	assert.Equal(t, "lt", state.Filters[0].Operator)
	assert.Equal(t, 20.0, state.Filters[0].Value)
}

func TestComprehension_PriceLessThan(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())

	state := &model.QueryState{
		NormalizedQuery: "less than 10 burger",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, 10.0, state.Filters[0].Value)
}

func TestComprehension_PriceCheaperThan(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())

	state := &model.QueryState{
		NormalizedQuery: "cheaper than 5.99 meal",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, 5.99, state.Filters[0].Value)
}

func TestComprehension_SortCheapest(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())

	state := &model.QueryState{
		NormalizedQuery: "cheapest burger",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.Sort)
	assert.Equal(t, "price", state.Sort.Field)
	assert.Equal(t, "asc", state.Sort.Direction)
}

func TestComprehension_SortNewest(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())

	state := &model.QueryState{
		NormalizedQuery: "newest arrivals",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.Sort)
	assert.Equal(t, "created_at", state.Sort.Field)
	assert.Equal(t, "desc", state.Sort.Direction)
}

func TestComprehension_PriceAndSort(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())

	state := &model.QueryState{
		NormalizedQuery: "cheapest burger under 20",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, 20.0, state.Filters[0].Value)
	require.NotNil(t, state.Sort)
	assert.Equal(t, "price", state.Sort.Field)
}

func TestComprehension_NoMatch(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())

	state := &model.QueryState{
		NormalizedQuery: "chicken burger",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Empty(t, state.Filters)
	assert.Nil(t, state.Sort)
}

func TestComprehension_EmptyConfig(t *testing.T) {
	step := NewComprehensionEngine(config.ComprehensionConfig{})

	state := &model.QueryState{
		NormalizedQuery: "cheapest burger under 20",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Empty(t, state.Filters)
	assert.Nil(t, state.Sort)
}
