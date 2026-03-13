package adaptive

import (
	"context"
	"testing"
	"time"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/hybrid"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/native"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func timeNow() time.Time { return time.Now() }

// stubV3 implements a minimal native pipeline for testing.
// We can't use native.Pipeline directly without real OS deps,
// so we test through the public Run method with a helper that
// wraps both pipelines.

// adaptiveTestHelper constructs an adaptive pipeline with mock v3/v2 runners.
type adaptiveTestHelper struct {
	v3Resp model.AnalyzeResponse
	v3Err  error
	v2Resp model.AnalyzeResponse
	v2Used bool
}

// To test the adaptive pipeline without real OS/LLM dependencies,
// we test the routing logic by calling Score + escalation decision directly.
// The Pipeline itself delegates to real v3/v2 instances, so integration
// testing requires those deps. Unit tests focus on the scoring and routing.

func TestPipeline_NoEscalation(t *testing.T) {
	// Simulate a v3 result that's good enough: high coverage, good concepts.
	v3Resp := model.AnalyzeResponse{
		OriginalQuery:   "chicken breast",
		NormalizedQuery: "chicken breast",
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
			{Value: "breast", Normalized: "breast", Position: 1},
		},
		Concepts: []model.ConceptMatch{
			{Label: "chicken breast", Score: 0.95, Start: 0, End: 1},
		},
	}

	cs := Score(v3Resp, "chicken breast", DefaultScorerConfig())
	assert.False(t, cs.Escalate, "well-covered query should not escalate")
	assert.InDelta(t, 0.0, cs.Score, 0.05)
}

func TestPipeline_EscalatesConversational(t *testing.T) {
	v3Resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "show", Normalized: "show", Position: 0},
			{Value: "me", Normalized: "me", Position: 1},
			{Value: "something", Normalized: "something", Position: 2},
			{Value: "easy", Normalized: "easy", Position: 3},
			{Value: "for", Normalized: "for", Position: 4},
			{Value: "dinner", Normalized: "dinner", Position: 5},
		},
		Concepts: []model.ConceptMatch{
			{Label: "dinner", Score: 0.9, Start: 5, End: 5},
		},
	}

	cs := Score(v3Resp, "show me something easy for dinner", DefaultScorerConfig())
	assert.True(t, cs.Conversational)
	assert.True(t, cs.Escalate, "conversational query should always escalate")
}

func TestPipeline_EscalatesZeroConcepts(t *testing.T) {
	v3Resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "healthy", Normalized: "healthy", Position: 0},
			{Value: "meal", Normalized: "meal", Position: 1},
			{Value: "prep", Normalized: "prep", Position: 2},
		},
	}

	cs := Score(v3Resp, "healthy meal prep", DefaultScorerConfig())
	assert.True(t, cs.Escalate, "3+ tokens with zero concepts/filters should escalate")
}

func TestPipeline_V2NilFallsBackToV3(t *testing.T) {
	// When v2 is nil, escalation should still return v3 result.
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	p := &Pipeline{
		v2:        nil,
		scorerCfg: DefaultScorerConfig(),
		logger:    logger,
	}

	v3Resp := model.AnalyzeResponse{
		NormalizedQuery: "test query",
		Tokens: []model.Token{
			{Value: "test", Normalized: "test", Position: 0},
		},
	}
	cs := ComplexityScore{Escalate: true, Score: 0.8}

	result := p.escalate(context.Background(), model.AnalyzeRequest{Query: "test query"}, v3Resp, cs, timeNow())
	assert.True(t, result.Escalated)
	assert.True(t, result.V2Fallback)
	assert.Equal(t, "test query", result.Response.NormalizedQuery)
}

func TestPipeline_NewPipelineDefaults(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		ScorerCfg: DefaultScorerConfig(),
	})
	require.NotNil(t, p)
	assert.NotNil(t, p.logger)
	assert.Nil(t, p.v2)
	assert.Nil(t, p.v3)
}

// Verify that the Pipeline struct accepts real v3/v2 types (compile check).
var _ = func() {
	_ = &Pipeline{
		v3: &native.Pipeline{},
		v2: &hybrid.Pipeline{},
	}
}
