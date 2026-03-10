package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hellofresh/qus/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type goldenTestCase struct {
	Name     string               `json:"name"`
	Input    goldenInput          `json:"input"`
	Expected model.AnalyzeResponse `json:"expected"`
}

type goldenInput struct {
	Query  string `json:"query"`
	Locale string `json:"locale"`
	Market string `json:"market"`
}

func TestGolden_NormalizeTokenize(t *testing.T) {
	pattern := filepath.Join("..", "..", "..", "testdata", "golden", "normalize_tokenize_*.json")
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, files, "no golden test files found matching %s", pattern)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	p := New(logger, nil, Normalizer{}, Tokenizer{})

	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)

		var tc goldenTestCase
		require.NoError(t, json.Unmarshal(data, &tc))

		t.Run(tc.Name, func(t *testing.T) {
			state := &model.QueryState{
				OriginalQuery:   tc.Input.Query,
				NormalizedQuery: tc.Input.Query,
			}

			err := p.Run(context.Background(), state, false)
			require.NoError(t, err)

			resp := state.ToResponse()

			assert.Equal(t, tc.Expected.OriginalQuery, resp.OriginalQuery)
			assert.Equal(t, tc.Expected.NormalizedQuery, resp.NormalizedQuery)
			assert.Equal(t, tc.Expected.Tokens, resp.Tokens)
			assert.Equal(t, tc.Expected.Rewrites, resp.Rewrites)
		})
	}
}
