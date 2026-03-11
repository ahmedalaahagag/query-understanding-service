package hybrid

import (
	"context"
	"fmt"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLMParser struct {
	result *LLMParseResult
	err    error
}

func (m *mockLLMParser) Parse(_ context.Context, _, _ string) (*LLMParseResult, error) {
	return m.result, m.err
}

func testPipeline(parser LLMParser, searcher opensearch.ConceptSearcher) *Pipeline {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	pb := &PromptBuilder{
		systemPromptTemplate: "test system prompt",
		filters: config.AllowedFiltersConfig{
			Filters: []config.AllowedFilter{
				{Field: "price", Operators: []string{"lt", "lte", "gt", "gte", "eq"}, Type: "number"},
			},
		},
		sorts: config.AllowedSortsConfig{
			Sorts: []config.AllowedSort{{Key: "price_asc"}, {Key: "price_desc"}},
		},
	}

	validator := NewValidator(pb.filters, pb.sorts, 0.65)
	resolver := NewConceptResolver(searcher, logger)

	return NewPipeline(PipelineConfig{
		Parser:          parser,
		PromptBuilder:   pb,
		Validator:       validator,
		ConceptResolver: resolver,
		Logger:          logger,
		FailOpen:        true,
	})
}

func TestPipeline_FullSuccess(t *testing.T) {
	parser := &mockLLMParser{
		result: &LLMParseResult{
			NormalizedQuery: "chicken burger",
			CandidateConcepts: []LLMCandidateConcept{
				{Label: "burger", Field: "product_type", Confidence: 0.94},
			},
			Filters: []LLMFilter{
				{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
			},
			Sort:       &LLMSort{Field: "price", Direction: "asc", Confidence: 0.8},
			Confidence: 0.89,
		},
	}

	searcher := &mockConceptSearcher{
		results: map[string][]opensearch.ConceptHit{
			"burger": {{ID: "pt_burger", Label: "burger", Field: "product_type", Score: 5.0, Source: "exact"}},
		},
	}

	p := testPipeline(parser, searcher)
	req := model.AnalyzeRequest{Query: "cheap chiken burger under 20", Locale: "en-GB", Market: "uk"}

	resp, debugInfo := p.Run(context.Background(), req, false)

	assert.Equal(t, "cheap chiken burger under 20", resp.OriginalQuery)
	assert.Equal(t, "chicken burger", resp.NormalizedQuery)
	require.Len(t, resp.Concepts, 1)
	assert.Equal(t, "pt_burger", resp.Concepts[0].ID)
	assert.Equal(t, "llm", resp.Concepts[0].Source)
	require.Len(t, resp.Filters, 1)
	assert.Equal(t, "price", resp.Filters[0].Field)
	assert.NotNil(t, resp.Sort)
	assert.Equal(t, "price", resp.Sort.Field)
	assert.Nil(t, debugInfo)
}

func TestPipeline_LLMFailure_Fallback(t *testing.T) {
	parser := &mockLLMParser{err: fmt.Errorf("connection refused")}
	p := testPipeline(parser, nil)

	req := model.AnalyzeRequest{Query: "chicken burger", Locale: "en-GB", Market: "uk"}
	resp, _ := p.Run(context.Background(), req, false)

	assert.Equal(t, "chicken burger", resp.OriginalQuery)
	assert.Equal(t, "chicken burger", resp.NormalizedQuery)
	assert.NotEmpty(t, resp.Tokens)
	assert.Empty(t, resp.Concepts)
	assert.Empty(t, resp.Filters)
	assert.Nil(t, resp.Sort)
	assert.Contains(t, resp.Warnings[0], "llm fallback")
}

func TestPipeline_LowConfidence_DropsSemantics(t *testing.T) {
	parser := &mockLLMParser{
		result: &LLMParseResult{
			NormalizedQuery: "test",
			Filters: []LLMFilter{
				{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
			},
			Confidence: 0.3, // below threshold
		},
	}

	p := testPipeline(parser, nil)
	req := model.AnalyzeRequest{Query: "test", Locale: "en-GB", Market: "uk"}

	resp, _ := p.Run(context.Background(), req, false)
	assert.Empty(t, resp.Filters)
	assert.Contains(t, resp.Warnings[0], "overall confidence")
}

func TestPipeline_UnresolvedConcept(t *testing.T) {
	parser := &mockLLMParser{
		result: &LLMParseResult{
			NormalizedQuery: "alien food",
			CandidateConcepts: []LLMCandidateConcept{
				{Label: "alien_thing", Field: "product_type", Confidence: 0.9},
			},
			Confidence: 0.89,
		},
	}

	searcher := &mockConceptSearcher{results: map[string][]opensearch.ConceptHit{}}
	p := testPipeline(parser, searcher)

	req := model.AnalyzeRequest{Query: "alien food", Locale: "en-GB", Market: "uk"}
	resp, _ := p.Run(context.Background(), req, false)

	assert.Empty(t, resp.Concepts)
	assert.Contains(t, resp.Warnings[0], "unresolved concept dropped")
}

func TestPipeline_DebugMode(t *testing.T) {
	parser := &mockLLMParser{
		result: &LLMParseResult{
			NormalizedQuery: "chicken burger",
			Confidence:      0.89,
		},
	}

	p := testPipeline(parser, nil)
	req := model.AnalyzeRequest{Query: "chicken burger", Locale: "en-GB", Market: "uk"}

	_, debugInfo := p.Run(context.Background(), req, true)

	require.NotNil(t, debugInfo)
	assert.NotEmpty(t, debugInfo.PreprocessedTokens)
	assert.NotNil(t, debugInfo.RawLLMOutput)
	assert.NotNil(t, debugInfo.ValidatedIntent)
	assert.False(t, debugInfo.FallbackUsed)
	assert.NotEmpty(t, debugInfo.Duration)
}

func TestPipeline_DebugMode_Fallback(t *testing.T) {
	parser := &mockLLMParser{err: fmt.Errorf("timeout")}
	p := testPipeline(parser, nil)

	req := model.AnalyzeRequest{Query: "test", Locale: "en-GB", Market: "uk"}
	_, debugInfo := p.Run(context.Background(), req, true)

	require.NotNil(t, debugInfo)
	assert.True(t, debugInfo.FallbackUsed)
}

func TestPipeline_InvalidFilterDropped(t *testing.T) {
	parser := &mockLLMParser{
		result: &LLMParseResult{
			NormalizedQuery: "burger under 20",
			Filters: []LLMFilter{
				{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
				{Field: "invented", Operator: "eq", Value: "x", Confidence: 0.9},
			},
			Confidence: 0.89,
		},
	}

	p := testPipeline(parser, nil)
	req := model.AnalyzeRequest{Query: "burger under 20", Locale: "en-GB", Market: "uk"}

	resp, _ := p.Run(context.Background(), req, false)
	assert.Len(t, resp.Filters, 1)
	assert.Equal(t, "price", resp.Filters[0].Field)
}
