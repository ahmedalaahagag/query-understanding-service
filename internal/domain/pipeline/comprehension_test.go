package pipeline

import (
	"context"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testComprehensionConfig() config.ComprehensionConfig {
	return config.ComprehensionConfig{
		"en": {
			FilterRules: []config.FilterRule{
				{Pattern: `(under|less than)\s+(\d+)\s*(minutes?|mins?)`, Field: "prep_time", Operator: "lt"},
				{Pattern: `\b(quick|fast)\b`, Field: "prep_time", Operator: "lte", Value: "30"},
				{Pattern: `(under|less than)\s+(\d+)\s*cal(ories?|s)?`, Field: "calories", Operator: "lt"},
				{Pattern: `\b(low calorie|low cal)\b`, Field: "calories", Operator: "lte", Value: "400"},
				{Pattern: `(under|less than|cheaper than)\s+(\d+(?:\.\d+)?)`, Field: "price", Operator: "lt"},
				{Pattern: `\b(easy|simple)\b`, Field: "difficulty_level", Operator: "eq", Value: "easy"},
				{Pattern: `\b(hard|difficult)\b`, Field: "difficulty_level", Operator: "eq", Value: "hard"},
				{Pattern: `(for|serves?)\s+(\d+)\s*(people|persons?)?`, Field: "servings", Operator: "eq"},
			},
			SortRules: []config.SortRule{
				{Pattern: `(cheapest|lowest price)`, Field: "price", Direction: "asc"},
				{Pattern: `(newest|most recent)`, Field: "created_at", Direction: "desc"},
				{Pattern: `(fastest|quickest)`, Field: "prep_time", Direction: "asc"},
			},
		},
		"de": {
			FilterRules: []config.FilterRule{
				{Pattern: `(unter|weniger als)\s+(\d+(?:[.,]\d+)?)`, Field: "price", Operator: "lt"},
				{Pattern: `\b(einfach|simpel)\b`, Field: "difficulty_level", Operator: "eq", Value: "easy"},
			},
			SortRules: []config.SortRule{
				{Pattern: `(günstigste|billigste)`, Field: "price", Direction: "asc"},
			},
		},
	}
}

func TestComprehension_PriceUnder(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "chicken burger under 20",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "price", state.Filters[0].Field)
	assert.Equal(t, "lt", state.Filters[0].Operator)
	assert.Equal(t, 20.0, state.Filters[0].Value)
}

func TestComprehension_PriceLessThan(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "less than 10 burger",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, 10.0, state.Filters[0].Value)
}

func TestComprehension_PriceCheaperThan(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "cheaper than 5.99 meal",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, 5.99, state.Filters[0].Value)
}

func TestComprehension_SortCheapest(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "cheapest burger",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.Sort)
	assert.Equal(t, "price", state.Sort.Field)
	assert.Equal(t, "asc", state.Sort.Direction)
}

func TestComprehension_SortNewest(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "newest arrivals",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.Sort)
	assert.Equal(t, "created_at", state.Sort.Field)
	assert.Equal(t, "desc", state.Sort.Direction)
}

func TestComprehension_PriceAndSort(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "cheapest burger under 20",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, 20.0, state.Filters[0].Value)
	require.NotNil(t, state.Sort)
	assert.Equal(t, "price", state.Sort.Field)
}

func TestComprehension_NoMatch(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "chicken burger",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Empty(t, state.Filters)
	assert.Nil(t, state.Sort)
}

func TestComprehension_EmptyConfig(t *testing.T) {
	step := NewComprehensionEngine(config.ComprehensionConfig{})
	state := &model.QueryState{
		NormalizedQuery: "cheapest burger under 20",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	assert.Empty(t, state.Filters)
	assert.Nil(t, state.Sort)
}

func TestComprehension_UnknownLocale(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "cheapest burger under 20",
		Locale:          "ja-JP",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	// No rules for Japanese — nothing extracted.
	assert.Empty(t, state.Filters)
	assert.Nil(t, state.Sort)
}

func TestComprehension_GermanLocale(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "hähnchen unter 10",
		Locale:          "de-DE",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "price", state.Filters[0].Field)
	assert.Equal(t, "lt", state.Filters[0].Operator)
	assert.Equal(t, 10.0, state.Filters[0].Value)
}

func TestComprehension_GermanSort(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "günstigste rezepte",
		Locale:          "de-DE",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.Sort)
	assert.Equal(t, "price", state.Sort.Field)
	assert.Equal(t, "asc", state.Sort.Direction)
}

func TestComprehension_KeywordDifficulty(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "easy chicken recipes",
		Locale:          "en-GB",
		Tokens: []model.Token{
			{Value: "easy", Normalized: "easy", Position: 0},
			{Value: "chicken", Normalized: "chicken", Position: 1},
			{Value: "recipes", Normalized: "recipes", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "difficulty_level", state.Filters[0].Field)
	assert.Equal(t, "eq", state.Filters[0].Operator)
	assert.Equal(t, "easy", state.Filters[0].Value)

	// "easy" should be consumed from tokens.
	assert.Equal(t, "chicken recipes", state.NormalizedQuery)
	assert.Len(t, state.Tokens, 2)
}

func TestComprehension_KeywordQuick(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "quick pasta dinner",
		Locale:          "en-GB",
		Tokens: []model.Token{
			{Value: "quick", Normalized: "quick", Position: 0},
			{Value: "pasta", Normalized: "pasta", Position: 1},
			{Value: "dinner", Normalized: "dinner", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "prep_time", state.Filters[0].Field)
	assert.Equal(t, "lte", state.Filters[0].Operator)
	assert.Equal(t, 30.0, state.Filters[0].Value) // numeric, not string "30"

	assert.Equal(t, "pasta dinner", state.NormalizedQuery)
}

func TestComprehension_PrepTimeMinutes(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "recipes under 15 minutes",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "prep_time", state.Filters[0].Field)
	assert.Equal(t, 15.0, state.Filters[0].Value)
}

func TestComprehension_CaloriesNumeric(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "chicken under 500 calories",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "calories", state.Filters[0].Field)
	assert.Equal(t, 500.0, state.Filters[0].Value)
}

func TestComprehension_ServingsForPeople(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "pasta for 4 people",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "servings", state.Filters[0].Field)
	assert.Equal(t, "eq", state.Filters[0].Operator)
	assert.Equal(t, 4.0, state.Filters[0].Value)
}

func TestComprehension_GermanDifficulty(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "einfach hähnchen rezepte",
		Locale:          "de-DE",
		Tokens: []model.Token{
			{Value: "einfach", Normalized: "einfach", Position: 0},
			{Value: "hähnchen", Normalized: "hähnchen", Position: 1},
			{Value: "rezepte", Normalized: "rezepte", Position: 2},
		},
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.Len(t, state.Filters, 1)
	assert.Equal(t, "difficulty_level", state.Filters[0].Field)
	assert.Equal(t, "easy", state.Filters[0].Value)
	assert.Equal(t, "hähnchen rezepte", state.NormalizedQuery)
}

func TestComprehension_SortFastest(t *testing.T) {
	step := NewComprehensionEngine(testComprehensionConfig())
	state := &model.QueryState{
		NormalizedQuery: "fastest chicken recipes",
		Locale:          "en-GB",
	}

	err := step.Process(context.Background(), state)
	require.NoError(t, err)

	require.NotNil(t, state.Sort)
	assert.Equal(t, "prep_time", state.Sort.Field)
	assert.Equal(t, "asc", state.Sort.Direction)
}
