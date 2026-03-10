package hybrid

import (
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestBuildFallbackResponse(t *testing.T) {
	state := &model.QueryState{
		OriginalQuery:   "Chicken Burger",
		NormalizedQuery: "chicken burger",
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
			{Value: "burger", Normalized: "burger", Position: 1},
		},
	}

	resp := BuildFallbackResponse(state, "timeout")
	assert.Equal(t, "Chicken Burger", resp.OriginalQuery)
	assert.Equal(t, "chicken burger", resp.NormalizedQuery)
	assert.Len(t, resp.Tokens, 2)
	assert.Equal(t, []string{"chicken burger"}, resp.Rewrites)
	assert.Empty(t, resp.Concepts)
	assert.Empty(t, resp.Filters)
	assert.Nil(t, resp.Sort)
	assert.Contains(t, resp.Warnings[0], "llm fallback: timeout")
}

func TestBuildFallbackResponse_NilTokens(t *testing.T) {
	state := &model.QueryState{
		OriginalQuery:   "test",
		NormalizedQuery: "test",
	}
	resp := BuildFallbackResponse(state, "error")
	assert.NotNil(t, resp.Tokens)
	assert.Empty(t, resp.Tokens)
}

func TestBuildFallbackResponse_NoRewrite(t *testing.T) {
	state := &model.QueryState{
		OriginalQuery:   "test",
		NormalizedQuery: "test",
	}
	resp := BuildFallbackResponse(state, "error")
	assert.Empty(t, resp.Rewrites)
}

func TestBuildFallbackResponse_WithFiltersAndSort(t *testing.T) {
	state := &model.QueryState{
		OriginalQuery:   "cheap chicken under 20",
		NormalizedQuery: "chicken",
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
		},
		Filters: []model.Filter{
			{Field: "price", Operator: "lt", Value: 2000.0},
		},
		Sort: &model.SortSpec{Field: "price", Direction: "asc"},
	}

	resp := BuildFallbackResponse(state, "timeout")
	assert.Equal(t, "chicken", resp.NormalizedQuery)
	assert.Len(t, resp.Tokens, 1)
	assert.Len(t, resp.Filters, 1)
	assert.Equal(t, "price", resp.Filters[0].Field)
	assert.NotNil(t, resp.Sort)
	assert.Equal(t, "price", resp.Sort.Field)
}
