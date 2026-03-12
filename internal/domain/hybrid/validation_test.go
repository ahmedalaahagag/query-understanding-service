package hybrid

import (
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/stretchr/testify/assert"
)

func testValidator() *Validator {
	return NewValidator(
		config.AllowedFiltersConfig{
			Filters: []config.AllowedFilter{
				{Field: "price", Operators: []string{"lt", "lte", "gt", "gte", "eq"}, Type: "number"},
				{Field: "dietary", Operators: []string{"eq", "in"}, Type: "string_list"},
			},
		},
		config.AllowedSortsConfig{
			Sorts: []config.AllowedSort{
				{Key: "relevance"},
				{Key: "price_asc"},
				{Key: "price_desc"},
				{Key: "newest"},
			},
		},
		0.65,
	)
}

func TestValidator_ValidFilters(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "chicken burger under 20",
		Filters: []LLMFilter{
			{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, "chicken burger under 20")
	assert.Len(t, intent.Filters, 1)
	assert.Equal(t, "price", intent.Filters[0].Field)
	assert.Empty(t, intent.Warnings)
}

func TestValidator_InvalidFilterField(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "test",
		Filters: []LLMFilter{
			{Field: "invented_field", Operator: "eq", Value: "foo", Confidence: 0.9},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, result.NormalizedQuery)
	assert.Empty(t, intent.Filters)
	assert.Contains(t, intent.Warnings[0], "not allowed")
}

func TestValidator_InvalidOperator(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "test",
		Filters: []LLMFilter{
			{Field: "dietary", Operator: "lt", Value: "vegan", Confidence: 0.9},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, result.NormalizedQuery)
	assert.Empty(t, intent.Filters)
	assert.Contains(t, intent.Warnings[0], "operator")
}

func TestValidator_LowConfidenceFilter(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "burger under 10",
		Filters: []LLMFilter{
			{Field: "price", Operator: "lt", Value: float64(10), Confidence: 0.3},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, "burger under 10")
	assert.Empty(t, intent.Filters)
	assert.Contains(t, intent.Warnings[0], "below threshold")
}

func TestValidator_ValidSort(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "test",
		Sort:            &LLMSort{Field: "price", Direction: "asc", Confidence: 0.8},
		Confidence:      0.89,
	}

	intent := v.Validate(result, result.NormalizedQuery)
	assert.NotNil(t, intent.Sort)
	assert.Equal(t, "price", intent.Sort.Field)
}

func TestValidator_InvalidSort(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "test",
		Sort:            &LLMSort{Field: "random", Direction: "asc", Confidence: 0.8},
		Confidence:      0.89,
	}

	intent := v.Validate(result, result.NormalizedQuery)
	assert.Nil(t, intent.Sort)
	assert.Contains(t, intent.Warnings[0], "sort")
}

func TestValidator_RelevanceSort(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "test",
		Sort:            &LLMSort{Field: "relevance", Direction: "desc", Confidence: 0.9},
		Confidence:      0.89,
	}

	intent := v.Validate(result, result.NormalizedQuery)
	assert.NotNil(t, intent.Sort)
}

func TestValidator_LowOverallConfidence(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "burger under 20",
		Filters: []LLMFilter{
			{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
		},
		Sort:       &LLMSort{Field: "price", Direction: "asc", Confidence: 0.8},
		Confidence: 0.3,
	}

	intent := v.Validate(result, "burger under 20")
	assert.Empty(t, intent.Filters)
	assert.Nil(t, intent.Sort)
	assert.Contains(t, intent.Warnings[0], "overall confidence")
}

func TestValidator_LowConfidenceConcept(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "test",
		CandidateConcepts: []LLMCandidateConcept{
			{Label: "burger", Field: "product_type", Confidence: 0.94},
			{Label: "maybe_thing", Field: "product_type", Confidence: 0.2},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, result.NormalizedQuery)
	assert.Len(t, intent.CandidateConcepts, 1)
	assert.Equal(t, "burger", intent.CandidateConcepts[0].Label)
}

func TestValidator_MixedValid_Invalid(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "vegan burger under 20",
		Filters: []LLMFilter{
			{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
			{Field: "bogus", Operator: "eq", Value: "x", Confidence: 0.9},
			{Field: "dietary", Operator: "eq", Value: "vegan", Confidence: 0.8},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, "vegan burger under 20")
	assert.Len(t, intent.Filters, 2)
	assert.Equal(t, "price", intent.Filters[0].Field)
	assert.Equal(t, "dietary", intent.Filters[1].Field)
	assert.Len(t, intent.Warnings, 1)
}

func TestValidator_GroundingDropsUnmentionedStringValue(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "chicken burger",
		Filters: []LLMFilter{
			{Field: "dietary", Operator: "eq", Value: "vegan", Confidence: 0.9},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, "chicken burger")
	assert.Empty(t, intent.Filters)
	assert.Contains(t, intent.Warnings[0], "not found in query")
}

func TestValidator_GroundingDropsUnmentionedNumber(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "chicken burger",
		Filters: []LLMFilter{
			{Field: "price", Operator: "lt", Value: float64(15), Confidence: 0.9},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, "chicken burger")
	assert.Empty(t, intent.Filters)
	assert.Contains(t, intent.Warnings[0], "not found in query")
}

func TestValidateNormalizedQuery_RevertsHallucinatedWord(t *testing.T) {
	result := validateNormalizedQuery(
		"i want something for my sunday ravioli with ravioli and vieggies",
		"I want something for my sunday party with pasta and vieggies",
		2,
	)
	// "ravioli" is not in the original and is >2 edits from any original word.
	assert.Equal(t, "i want something for my sunday party with pasta and vieggies", result.query)
	assert.Len(t, result.warnings, 1)
	assert.Contains(t, result.warnings[0], "ravioli")
}

func TestValidateNormalizedQuery_AllowsTypoCorrection(t *testing.T) {
	result := validateNormalizedQuery(
		"i want chicken burger",
		"I want chiken burger",
		2,
	)
	// "chicken" is within 1 edit of "chiken" — allowed.
	assert.Equal(t, "i want chicken burger", result.query)
	assert.Empty(t, result.warnings)
}

func TestValidateNormalizedQuery_RejectsTwoEditOnShortWord(t *testing.T) {
	result := validateNormalizedQuery(
		"i want something for my sunday pasta with pasta and vieggies",
		"I want something for my sunday party with pasta and vieggies",
		2,
	)
	// "pasta" replacing "party" is 2 edits on a 5-char word — rejected.
	assert.Equal(t, "i want something for my sunday party with pasta and vieggies", result.query)
	assert.Len(t, result.warnings, 1)
}

func TestValidateNormalizedQuery_IdenticalQuery(t *testing.T) {
	result := validateNormalizedQuery("chicken burger", "chicken burger", 2)
	assert.Equal(t, "chicken burger", result.query)
	assert.Empty(t, result.warnings)
}

func TestValidateNormalizedQuery_EmptyLLMQuery(t *testing.T) {
	result := validateNormalizedQuery("", "chicken burger", 2)
	assert.Equal(t, "", result.query)
	assert.Empty(t, result.warnings)
}

func TestValidator_NormalizedQueryValidation(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "i want ravioli with ravioli",
		Confidence:      0.89,
	}

	intent := v.Validate(result, "i want pasta with pasta")
	// Should revert to original since "ravioli" is a hallucination.
	assert.Equal(t, "i want pasta with pasta", intent.NormalizedQuery)
	assert.Contains(t, intent.Warnings[0], "ravioli")
}

func TestValidator_GroundingKeepsValueFoundInQuery(t *testing.T) {
	v := testValidator()
	result := &LLMParseResult{
		NormalizedQuery: "vegan burger under 20",
		Filters: []LLMFilter{
			{Field: "dietary", Operator: "eq", Value: "vegan", Confidence: 0.9},
			{Field: "price", Operator: "lt", Value: float64(20), Confidence: 0.96},
		},
		Confidence: 0.89,
	}

	intent := v.Validate(result, "vegan burger under 20")
	assert.Len(t, intent.Filters, 2)
	assert.Empty(t, intent.Warnings)
}
