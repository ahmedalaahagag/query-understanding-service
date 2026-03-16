package adaptive

import (
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/hybrid"
	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/native"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPipeline_Defaults(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		DirectLLMTokenThreshold: 3,
	})
	require.NotNil(t, p)
	assert.NotNil(t, p.logger)
	assert.Equal(t, 3, p.directLLMTokenThreshold)
	assert.Nil(t, p.v2)
	assert.Nil(t, p.v3)
}

func TestCountNonStopwordTokens(t *testing.T) {
	stopwords := map[string]map[string]bool{
		"en_gb": {"the": true, "for": true, "with": true, "and": true, "a": true},
	}

	p := &Pipeline{stopwords: stopwords, logger: logrus.New()}

	tests := []struct {
		name   string
		query  string
		locale string
		want   int
	}{
		{"no stopwords", "chicken breast", "en-GB", 2},
		{"with stopwords", "chicken with rice and vegetables", "en-GB", 3},
		{"all stopwords", "the for with", "en-GB", 0},
		{"unknown locale", "chicken for dinner", "de-DE", 3},
		{"empty query", "", "en-GB", 0},
		{"three non-stopword tokens", "healthy meal prep for the week", "en-GB", 4},
		{"exactly at threshold", "chicken soup for the soul", "en-GB", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.countNonStopwordTokens(tt.query, tt.locale)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCountNonStopwordTokens_NilStopwords(t *testing.T) {
	p := &Pipeline{logger: logrus.New()}
	got := p.countNonStopwordTokens("chicken for dinner", "en-GB")
	assert.Equal(t, 3, got, "nil stopwords should count all words")
}

func TestCountNonStopwordTokens_ThresholdRouting(t *testing.T) {
	stopwords := map[string]map[string]bool{
		"en_gb": {"the": true, "for": true, "with": true, "and": true, "a": true, "me": true},
	}
	p := &Pipeline{stopwords: stopwords, logger: logrus.New()}

	// "chicken" = 1 token → below threshold 3 → v3
	assert.Less(t, p.countNonStopwordTokens("chicken", "en-GB"), 3)

	// "chicken breast" = 2 tokens → below threshold 3 → v3
	assert.Less(t, p.countNonStopwordTokens("chicken breast", "en-GB"), 3)

	// "healthy meal prep" = 3 tokens → at threshold → v2
	assert.GreaterOrEqual(t, p.countNonStopwordTokens("healthy meal prep", "en-GB"), 3)

	// "show me something easy for dinner" = 4 non-stopwords → v2
	assert.GreaterOrEqual(t, p.countNonStopwordTokens("show me something easy for dinner", "en-GB"), 3)
}

// Verify that the Pipeline struct accepts real v3/v2 types (compile check).
var _ = func() {
	_ = &Pipeline{
		v3: &native.Pipeline{},
		v2: &hybrid.Pipeline{},
	}
}
