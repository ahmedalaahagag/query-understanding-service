package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/hellofresh/qus/internal/domain/hybrid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.ErrorLevel)
	return l
}

func mockClient(fn converseFunc) *Client {
	return &Client{
		converse: fn,
		cfg:      ClientConfig{ModelID: "anthropic.claude-haiku-4-5-20251001", MaxRetries: 0},
		logger:   testLogger(),
	}
}

func converseOK(jsonContent string) converseFunc {
	return func(_ context.Context, _ *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
		return &bedrockruntime.ConverseOutput{
			Output: &types.ConverseOutputMemberMessage{
				Value: types.Message{
					Content: []types.ContentBlock{
						&types.ContentBlockMemberText{Value: jsonContent},
					},
				},
			},
		}, nil
	}
}

func converseErr(err error) converseFunc {
	return func(_ context.Context, _ *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
		return nil, err
	}
}

func TestClient_Parse_Success(t *testing.T) {
	llmResult := hybrid.LLMParseResult{
		NormalizedQuery: "chicken burger",
		CandidateConcepts: []hybrid.LLMCandidateConcept{
			{Label: "burger", Field: "product_type", Confidence: 0.94},
		},
		Filters: []hybrid.LLMFilter{
			{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
		},
		Confidence: 0.89,
	}
	llmJSON, _ := json.Marshal(llmResult)

	c := mockClient(converseOK(string(llmJSON)))
	result, err := c.Parse(context.Background(), "system", "cheap chicken burger under 20")
	require.NoError(t, err)
	assert.Equal(t, "chicken burger", result.NormalizedQuery)
	assert.Len(t, result.CandidateConcepts, 1)
	assert.Len(t, result.Filters, 1)
	assert.Equal(t, 0.89, result.Confidence)
}

func TestClient_Parse_InvalidJSON(t *testing.T) {
	c := mockClient(converseOK("not json at all"))
	_, err := c.Parse(context.Background(), "system", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing LLM JSON output")
}

func TestClient_Parse_APIError(t *testing.T) {
	c := mockClient(converseErr(fmt.Errorf("throttling exception")))
	_, err := c.Parse(context.Background(), "system", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "throttling exception")
}

func TestClient_Parse_EmptyContent(t *testing.T) {
	fn := func(_ context.Context, _ *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
		return &bedrockruntime.ConverseOutput{
			Output: &types.ConverseOutputMemberMessage{
				Value: types.Message{Content: []types.ContentBlock{}},
			},
		}, nil
	}
	c := mockClient(fn)
	_, err := c.Parse(context.Background(), "system", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected converse output")
}

func TestClient_Parse_RetryOnFailure(t *testing.T) {
	callCount := 0
	llmResult := hybrid.LLMParseResult{NormalizedQuery: "test", Confidence: 0.8}
	llmJSON, _ := json.Marshal(llmResult)

	fn := func(_ context.Context, _ *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("temporary error")
		}
		return &bedrockruntime.ConverseOutput{
			Output: &types.ConverseOutputMemberMessage{
				Value: types.Message{
					Content: []types.ContentBlock{
						&types.ContentBlockMemberText{Value: string(llmJSON)},
					},
				},
			},
		}, nil
	}

	c := &Client{converse: fn, cfg: ClientConfig{ModelID: "test", MaxRetries: 1}, logger: testLogger()}
	result, err := c.Parse(context.Background(), "system", "test")
	require.NoError(t, err)
	assert.Equal(t, "test", result.NormalizedQuery)
	assert.Equal(t, 2, callCount)
}
