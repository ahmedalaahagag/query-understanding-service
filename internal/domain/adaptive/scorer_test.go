package adaptive

import (
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestScore_SimpleQuery(t *testing.T) {
	// "chicken" → 1 token, 1 concept covering it, high score.
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
		},
		Concepts: []model.ConceptMatch{
			{Label: "chicken", Score: 0.95, Start: 0, End: 0},
		},
	}

	cs := Score(resp, "chicken", DefaultScorerConfig())

	assert.Equal(t, 1.0, cs.TokenCoverage)
	assert.Equal(t, 0.95, cs.AvgConceptScore)
	assert.Equal(t, 0, cs.SpellCorrections)
	assert.False(t, cs.Conversational)
	assert.InDelta(t, 0.0, cs.Score, 0.01)
	assert.False(t, cs.Escalate)
}

func TestScore_NoConcepts_FewTokens(t *testing.T) {
	// "ab" → 1 token, no concepts, no filters — but only 1 token so hard trigger doesn't fire.
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "ab", Normalized: "ab", Position: 0},
		},
	}

	cs := Score(resp, "ab", DefaultScorerConfig())
	assert.Equal(t, 0.0, cs.TokenCoverage)
	assert.InDelta(t, 0.4, cs.Score, 0.01) // full uncovered weight
	assert.False(t, cs.Escalate)            // score ≤ 0.5 and only 1 token
}

func TestScore_NoConcepts_ManyTokens(t *testing.T) {
	// 3+ tokens with zero concepts and zero filters → hard escalation trigger.
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "something", Normalized: "something", Position: 0},
			{Value: "easy", Normalized: "easy", Position: 1},
			{Value: "tonight", Normalized: "tonight", Position: 2},
		},
	}

	cs := Score(resp, "something easy tonight", DefaultScorerConfig())
	assert.True(t, cs.Escalate, "3+ tokens with no concepts/filters should escalate")
}

func TestScore_Conversational(t *testing.T) {
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "show", Normalized: "show", Position: 0},
			{Value: "me", Normalized: "me", Position: 1},
			{Value: "something", Normalized: "something", Position: 2},
			{Value: "quick", Normalized: "quick", Position: 3},
			{Value: "and", Normalized: "and", Position: 4},
			{Value: "easy", Normalized: "easy", Position: 5},
		},
	}

	cs := Score(resp, "show me something quick and easy", DefaultScorerConfig())
	assert.True(t, cs.Conversational)
	assert.True(t, cs.Escalate, "conversational queries always escalate")
}

func TestScore_ConversationalAfterStripping(t *testing.T) {
	// Reproduces the "show me something easy for dinner" → "ginger" bug.
	// v3 strips stopwords + comprehension, leaving only 1 token.
	// Conversational check must use original query word count, not post-processed.
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "dinner", Normalized: "ginger", Position: 0},
		},
		Concepts: []model.ConceptMatch{
			{Label: "ginger", Score: 0.85, Start: 0, End: 0},
		},
		Filters: []model.Filter{
			{Field: "difficulty_level", Operator: "eq", Value: 1},
		},
	}

	cs := Score(resp, "show me something easy for dinner", DefaultScorerConfig())
	assert.Equal(t, 6, cs.OriginalTokenCount)
	assert.Equal(t, 1, cs.TokenCount)
	assert.True(t, cs.Conversational, "should detect conversational from original query")
	assert.True(t, cs.Escalate, "conversational query with heavy stripping must escalate")
}

func TestScore_HeavyTokenReduction(t *testing.T) {
	// 4+ original words reduced to 1 token → heavy reduction trigger.
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "ideas", Normalized: "ideas", Position: 0},
		},
	}

	cs := Score(resp, "healthy quick dinner ideas", DefaultScorerConfig())
	assert.Equal(t, 4, cs.OriginalTokenCount)
	assert.Equal(t, 1, cs.TokenCount)
	assert.True(t, cs.Escalate, "4 words reduced to 1 token should escalate")
}

func TestScore_SpellCorrections(t *testing.T) {
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "chikken", Normalized: "chicken", Position: 0},
			{Value: "brest", Normalized: "breast", Position: 1},
			{Value: "recipee", Normalized: "recipe", Position: 2},
		},
		Concepts: []model.ConceptMatch{
			{Label: "chicken breast", Score: 0.8, Start: 0, End: 1},
		},
	}

	cs := Score(resp, "chikken brest recipee", DefaultScorerConfig())
	assert.Equal(t, 3, cs.SpellCorrections)
	assert.True(t, cs.SpellCorrections > DefaultScorerConfig().MaxSpellCorrections)
}

func TestScore_LowConceptScore(t *testing.T) {
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "pasta", Normalized: "pasta", Position: 0},
		},
		Concepts: []model.ConceptMatch{
			{Label: "pasta", Score: 0.5, Start: 0, End: 0},
		},
	}

	cs := Score(resp, "pasta", DefaultScorerConfig())
	assert.InDelta(t, 0.2, cs.Score, 0.01) // low concept score adds 0.2
}

func TestScore_PartialCoverage(t *testing.T) {
	// 2 tokens, only 1 covered → 50% coverage, at the threshold boundary.
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "chicken", Normalized: "chicken", Position: 0},
			{Value: "tonight", Normalized: "tonight", Position: 1},
		},
		Concepts: []model.ConceptMatch{
			{Label: "chicken", Score: 0.9, Start: 0, End: 0},
		},
	}

	cs := Score(resp, "chicken tonight", DefaultScorerConfig())
	assert.InDelta(t, 0.5, cs.TokenCoverage, 0.01)
}

func TestScore_FiltersPreventHardTrigger(t *testing.T) {
	// 3 tokens, 0 concepts, but has filters → hard trigger should NOT fire.
	resp := model.AnalyzeResponse{
		Tokens: []model.Token{
			{Value: "under", Normalized: "under", Position: 0},
			{Value: "30", Normalized: "30", Position: 1},
			{Value: "minutes", Normalized: "minutes", Position: 2},
		},
		Filters: []model.Filter{
			{Field: "preparation_time", Operator: "lte", Value: 1800},
		},
	}

	cs := Score(resp, "under 30 minutes", DefaultScorerConfig())
	// Has filters, so the hard trigger (0 concepts + 0 filters + 3 tokens) won't fire.
	assert.False(t, cs.ConceptCount == 0 && cs.FilterCount == 0 && cs.TokenCount >= 3)
}

func TestCountSpellCorrections(t *testing.T) {
	tokens := []model.Token{
		{Value: "chicken", Normalized: "chicken"},   // no correction
		{Value: "Chicken", Normalized: "chicken"},   // case only — no correction
		{Value: "chikken", Normalized: "chicken"},   // corrected
		{Value: "brest", Normalized: "breast"},      // corrected
	}

	assert.Equal(t, 2, countSpellCorrections(tokens))
}
