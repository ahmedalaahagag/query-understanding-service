package pipeline

import (
	"context"
	"testing"

	"github.com/hellofresh/qus/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmbiguityResolver_PrefersLongestSpan(t *testing.T) {
	step := AmbiguityResolver{}

	state := &model.QueryState{
		Concepts: []model.ConceptMatch{
			{ID: "c_chicken", Label: "chicken", MatchedText: "chicken", Score: 1.0, Start: 0, End: 0},
			{ID: "c_burger", Label: "burger", MatchedText: "burger", Score: 1.0, Start: 1, End: 1},
			{ID: "c_chicken_burger", Label: "chicken burger", MatchedText: "chicken burger", Score: 1.0, Start: 0, End: 1},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// "chicken burger" (span 0-1) beats individual "chicken" and "burger"
	require.Len(t, state.Concepts, 1)
	assert.Equal(t, "c_chicken_burger", state.Concepts[0].ID)
}

func TestAmbiguityResolver_PrefersHigherScore(t *testing.T) {
	step := AmbiguityResolver{}

	state := &model.QueryState{
		Concepts: []model.ConceptMatch{
			{ID: "c1", Label: "burger", MatchedText: "burger", Score: 0.7, Source: "spell", Start: 0, End: 0},
			{ID: "c2", Label: "burger", MatchedText: "burger", Score: 1.0, Source: "exact", Start: 0, End: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Concepts, 1)
	assert.Equal(t, "c2", state.Concepts[0].ID)
}

func TestAmbiguityResolver_NonOverlapping(t *testing.T) {
	step := AmbiguityResolver{}

	state := &model.QueryState{
		Concepts: []model.ConceptMatch{
			{ID: "c_cheap", Label: "cheap", MatchedText: "cheap", Score: 0.5, Start: 0, End: 0},
			{ID: "c_burger", Label: "burger", MatchedText: "burger", Score: 1.0, Start: 2, End: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// Non-overlapping: both survive
	assert.Len(t, state.Concepts, 2)
}

func TestAmbiguityResolver_Empty(t *testing.T) {
	step := AmbiguityResolver{}

	state := &model.QueryState{
		Concepts: []model.ConceptMatch{},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)
	assert.Empty(t, state.Concepts)
}

func TestAmbiguityResolver_Single(t *testing.T) {
	step := AmbiguityResolver{}

	state := &model.QueryState{
		Concepts: []model.ConceptMatch{
			{ID: "c1", Label: "burger", Score: 1.0, Start: 0, End: 0},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)
	assert.Len(t, state.Concepts, 1)
}

func TestAmbiguityResolver_ComplexOverlap(t *testing.T) {
	step := AmbiguityResolver{}

	// "cheap chicken burger" with concepts overlapping at different positions
	state := &model.QueryState{
		Concepts: []model.ConceptMatch{
			{ID: "c_cheap", Label: "cheap", MatchedText: "cheap", Score: 0.5, Start: 0, End: 0},
			{ID: "c_chicken", Label: "chicken", MatchedText: "chicken", Score: 0.9, Start: 1, End: 1},
			{ID: "c_burger", Label: "burger", MatchedText: "burger", Score: 1.0, Start: 2, End: 2},
			{ID: "c_chicken_burger", Label: "chicken burger", MatchedText: "chicken burger", Score: 0.95, Start: 1, End: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// "chicken burger" (span 1-2, score 0.95) beats individual chicken + burger
	// "cheap" (span 0) is non-overlapping, so it survives
	ids := make([]string, len(state.Concepts))
	for i, c := range state.Concepts {
		ids[i] = c.ID
	}
	assert.Contains(t, ids, "c_chicken_burger")
	assert.Contains(t, ids, "c_cheap")
	assert.Len(t, state.Concepts, 2)
}
