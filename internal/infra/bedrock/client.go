package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/hellofresh/qus/internal/domain/hybrid"
	"github.com/sirupsen/logrus"
)

// converseFunc abstracts the Bedrock Converse call for testability.
type converseFunc func(ctx context.Context, input *bedrockruntime.ConverseInput, opts ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)

// ClientConfig holds Bedrock connection settings.
type ClientConfig struct {
	Region     string
	ModelID    string
	MaxRetries int
}

// Client calls Bedrock Converse API for structured LLM output.
type Client struct {
	converse converseFunc
	cfg      ClientConfig
	logger   *logrus.Logger
}

// NewClient creates a new Bedrock client using the default AWS credential chain.
func NewClient(ctx context.Context, cfg ClientConfig, logger *logrus.Logger) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	brClient := bedrockruntime.NewFromConfig(awsCfg)

	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1
	}

	return &Client{
		converse: brClient.Converse,
		cfg:      cfg,
		logger:   logger,
	}, nil
}

// Parse calls Claude via Bedrock Converse API and parses the response into LLMParseResult.
func (c *Client) Parse(ctx context.Context, systemPrompt, userMessage string) (*hybrid.LLMParseResult, error) {
	c.logger.WithFields(logrus.Fields{
		"model":        c.cfg.ModelID,
		"region":       c.cfg.Region,
		"user_message": userMessage,
	}).Info("sending bedrock converse request")

	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			c.logger.WithField("attempt", attempt+1).Debug("retrying bedrock converse")
		}
		result, err := c.doConverse(ctx, systemPrompt, userMessage)
		if err == nil {
			c.logger.WithField("confidence", result.Confidence).Debug("bedrock converse succeeded")
			return result, nil
		}
		lastErr = err
		c.logger.WithError(err).WithField("attempt", attempt+1).Warn("bedrock converse attempt failed")
	}
	return nil, fmt.Errorf("bedrock call failed after %d attempts: %w", c.cfg.MaxRetries+1, lastErr)
}

func (c *Client) doConverse(ctx context.Context, systemPrompt, userMessage string) (*hybrid.LLMParseResult, error) {
	input := &bedrockruntime.ConverseInput{
		ModelId: aws.String(c.cfg.ModelID),
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: systemPrompt},
		},
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: userMessage},
				},
			},
		},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(256),
			Temperature: aws.Float32(0.0),
		},
	}

	output, err := c.converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("converse call: %w", err)
	}

	msgOutput, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok || len(msgOutput.Value.Content) == 0 {
		return nil, fmt.Errorf("unexpected converse output format")
	}

	textBlock, ok := msgOutput.Value.Content[0].(*types.ContentBlockMemberText)
	if !ok {
		return nil, fmt.Errorf("expected text content block, got different type")
	}

	c.logger.WithField("raw_response", textBlock.Value).Info("bedrock raw LLM response")

	raw := stripMarkdownFences(textBlock.Value)

	var result hybrid.LLMParseResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parsing LLM JSON output: %w", err)
	}

	return &result, nil
}

// stripMarkdownFences removes ```json ... ``` wrapping that models sometimes add.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence (```json or ```)
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}
