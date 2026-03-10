package hybrid

import (
	"testing"

	"github.com/hellofresh/qus/internal/domain/model"
	"github.com/stretchr/testify/assert"
)

func TestBuildFallbackResponse(t *testing.T) {
	tokens := []model.Token{
		{Value: "chicken", Normalized: "chicken", Position: 0},
		{Value: "burger", Normalized: "burger", Position: 1},
	}

	resp := BuildFallbackResponse("Chicken Burger", "chicken burger", tokens, "timeout")
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
	resp := BuildFallbackResponse("test", "test", nil, "error")
	assert.NotNil(t, resp.Tokens)
	assert.Empty(t, resp.Tokens)
}

func TestBuildFallbackResponse_NoRewrite(t *testing.T) {
	resp := BuildFallbackResponse("test", "test", nil, "error")
	assert.Empty(t, resp.Rewrites)
}
