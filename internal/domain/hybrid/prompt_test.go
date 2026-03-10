package hybrid

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptBuilder_SystemPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	require.NoError(t, os.WriteFile(promptFile, []byte("You are a parser."), 0644))

	pb, err := NewPromptBuilder(promptFile,
		config.AllowedFiltersConfig{
			Filters: []config.AllowedFilter{
				{Field: "price", Operators: []string{"lt", "gt"}, Type: "number"},
			},
		},
		config.AllowedSortsConfig{
			Sorts: []config.AllowedSort{{Key: "price_asc"}},
		},
	)
	require.NoError(t, err)

	prompt := pb.SystemPrompt()
	assert.Contains(t, prompt, "You are a parser.")
	assert.Contains(t, prompt, "ALLOWED FILTERS:")
	assert.Contains(t, prompt, "price")
	assert.Contains(t, prompt, "lt,gt")
	assert.Contains(t, prompt, "[number]")
	assert.Contains(t, prompt, "ALLOWED SORTS:")
	assert.Contains(t, prompt, "price_asc")
}

func TestPromptBuilder_SystemPrompt_WithDescriptionAndExamples(t *testing.T) {
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	require.NoError(t, os.WriteFile(promptFile, []byte("Parse query."), 0644))

	pb, err := NewPromptBuilder(promptFile,
		config.AllowedFiltersConfig{
			Filters: []config.AllowedFilter{
				{
					Field:       "recipe_cuisine",
					Operators:   []string{"eq", "in"},
					Type:        "keyword",
					Description: "Cuisine origin of the recipe. NOT a food type.",
					Examples:    []string{"Italian", "Mexican", "Asian"},
				},
			},
		},
		config.AllowedSortsConfig{},
	)
	require.NoError(t, err)

	prompt := pb.SystemPrompt()
	assert.Contains(t, prompt, "Cuisine origin of the recipe. NOT a food type.")
	assert.Contains(t, prompt, "e.g. Italian, Mexican, Asian")
}

func TestPromptBuilder_UserMessage(t *testing.T) {
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	require.NoError(t, os.WriteFile(promptFile, []byte("system"), 0644))

	pb, err := NewPromptBuilder(promptFile,
		config.AllowedFiltersConfig{},
		config.AllowedSortsConfig{},
	)
	require.NoError(t, err)

	msg := pb.UserMessage("chicken burger", "en-GB", "uk")
	assert.Contains(t, msg, "Query: chicken burger")
	assert.Contains(t, msg, "Locale: en-GB")
	assert.Contains(t, msg, "Market: uk")
}
