package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// e2eSpellChecker returns predefined corrections for known misspellings.
type e2eSpellChecker struct {
	corrections map[string]opensearch.SpellSuggestion
}

func (m *e2eSpellChecker) Suggest(_ context.Context, text, _ string) ([]opensearch.SpellSuggestion, error) {
	if s, ok := m.corrections[text]; ok {
		return []opensearch.SpellSuggestion{s}, nil
	}
	return nil, nil
}

// e2eLinguisticLookup returns predefined linguistic matches.
type e2eLinguisticLookup struct {
	entries map[string][]opensearch.LinguisticMatch
}

func (m *e2eLinguisticLookup) Lookup(_ context.Context, term, _ string) ([]opensearch.LinguisticMatch, error) {
	if matches, ok := m.entries[term]; ok {
		return matches, nil
	}
	return nil, nil
}

// e2eCompoundLookup returns predefined compound entries.
type e2eCompoundLookup struct {
	entries map[string][]opensearch.CompoundEntry
}

func (m *e2eCompoundLookup) LookupCompounds(_ context.Context, text, _ string) ([]opensearch.CompoundEntry, error) {
	if entries, ok := m.entries[text]; ok {
		return entries, nil
	}
	return nil, nil
}

// e2eConceptSearcher returns predefined concept hits for known queries.
type e2eConceptSearcher struct {
	concepts map[string][]opensearch.ConceptHit
}

func (m *e2eConceptSearcher) SearchConcepts(_ context.Context, text, _, _ string) ([]opensearch.ConceptHit, error) {
	if hits, ok := m.concepts[text]; ok {
		return hits, nil
	}
	return nil, nil
}

type e2eGoldenTestCase struct {
	Name     string               `json:"name"`
	Input    e2eGoldenInput       `json:"input"`
	Expected model.AnalyzeResponse `json:"expected"`
}

type e2eGoldenInput struct {
	Query  string `json:"query"`
	Locale string `json:"locale"`
	Market string `json:"market"`
}

func newE2EPipeline() *Pipeline {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	spellChecker := &e2eSpellChecker{
		corrections: map[string]opensearch.SpellSuggestion{
			"chiken":    {Text: "chicken", Score: 0.95},
			"burgar":    {Text: "burger", Score: 0.92},
			"pizzza":    {Text: "pizza", Score: 0.90},
			"cheeze":    {Text: "cheese", Score: 0.91},
			"vegitable": {Text: "vegetable", Score: 0.88},
			"tomatoe":   {Text: "tomato", Score: 0.93},
		},
	}

	linguisticLookup := &e2eLinguisticLookup{
		entries: map[string][]opensearch.LinguisticMatch{
			"sneakers": {{Term: "trainers", Type: "SYN"}},
			"eggplant": {{Term: "aubergine", Type: "SYN"}},
			"veggie":   {{Term: "vegetarian", Type: "SYN"}},
			"veg":      {{Term: "vegetarian", Type: "SYN"}},
			"coke":     {{Term: "coca cola", Type: "SYN"}},
			"chips":    {{Term: "crisps", Type: "SYN"}},
		},
	}

	compoundLookup := &e2eCompoundLookup{
		entries: map[string][]opensearch.CompoundEntry{
			"ice cream":     {{Compound: "icecream", Parts: "ice cream"}},
			"peanut butter": {{Compound: "peanutbutter", Parts: "peanut butter"}},
			"crewneck":      {{Compound: "crewneck", Parts: "crew neck"}},
			"lunchbox":      {{Compound: "lunchbox", Parts: "lunch box"}},
		},
	}

	conceptSearcher := &e2eConceptSearcher{
		concepts: map[string][]opensearch.ConceptHit{
			"chicken burger": {
				{ID: "c-chicken-burger", Label: "chicken burger", Field: "category", Weight: 10, Score: 5.0, Source: "exact"},
			},
			"chicken": {
				{ID: "c-chicken", Label: "chicken", Field: "protein", Weight: 8, Score: 3.0, Source: "exact"},
			},
			"burger": {
				{ID: "c-burger", Label: "burger", Field: "category", Weight: 9, Score: 3.5, Source: "exact"},
			},
			"pizza": {
				{ID: "c-pizza", Label: "pizza", Field: "category", Weight: 9, Score: 4.0, Source: "exact"},
			},
			"cheese pizza": {
				{ID: "c-cheese-pizza", Label: "cheese pizza", Field: "category", Weight: 10, Score: 5.5, Source: "exact"},
			},
			"vegetarian": {
				{ID: "c-vegetarian", Label: "vegetarian", Field: "diet", Weight: 7, Score: 3.0, Source: "exact"},
			},
			"trainers": {
				{ID: "c-trainers", Label: "trainers", Field: "category", Weight: 8, Score: 4.0, Source: "exact"},
			},
			"icecream": {
				{ID: "c-icecream", Label: "icecream", Field: "category", Weight: 9, Score: 4.5, Source: "exact"},
			},
			"aubergine": {
				{ID: "c-aubergine", Label: "aubergine", Field: "ingredient", Weight: 7, Score: 3.5, Source: "exact"},
			},
			"crew neck": {
				{ID: "c-crew-neck", Label: "crew neck", Field: "category", Weight: 8, Score: 4.0, Source: "exact"},
			},
			"crew": {
				{ID: "c-crew", Label: "crew", Field: "category", Weight: 5, Score: 2.0, Source: "exact"},
			},
			"neck": {
				{ID: "c-neck", Label: "neck", Field: "category", Weight: 4, Score: 1.5, Source: "exact"},
			},
			"soup": {
				{ID: "c-soup", Label: "soup", Field: "category", Weight: 8, Score: 3.5, Source: "exact"},
			},
			"tomato soup": {
				{ID: "c-tomato-soup", Label: "tomato soup", Field: "category", Weight: 10, Score: 5.0, Source: "exact"},
			},
			"tomato": {
				{ID: "c-tomato", Label: "tomato", Field: "ingredient", Weight: 7, Score: 3.0, Source: "exact"},
			},
			"pasta": {
				{ID: "c-pasta", Label: "pasta", Field: "category", Weight: 9, Score: 4.0, Source: "exact"},
			},
		},
	}

	spellCfg := config.SpellConfig{
		Enabled:             true,
		MinTokenLength:      4,
		ConfidenceThreshold: 0.85,
	}

	conceptCfg := config.ConceptConfig{
		ShingleMaxSize:    4,
		MaxMatchesPerSpan: 3,
	}

	stopwords := map[string]map[string]bool{
		"en_gb": {"the": true, "a": true, "an": true, "for": true, "with": true, "and": true, "of": true, "me": true, "something": true, "some": true},
		"de_de": {"der": true, "die": true, "das": true, "und": true, "mit": true, "für": true, "ein": true, "eine": true},
	}

	comprehensionCfg := config.ComprehensionConfig{
		"en": {
			FilterRules: []config.FilterRule{
				// Prep time (before price — more specific)
				{Pattern: `(under|less than)\s+(\d+)\s*(minutes?|mins?)`, Field: "preparation_time", Operator: "lt", Multiplier: 60},
				{Pattern: `\b(super\s*quick|superquick)\b`, Field: "preparation_time", Operator: "lte", Value: "1200"},
				{Pattern: `\b(quick|fast)\b`, Field: "preparation_time", Operator: "lte", Value: "1800"},
				// Price (generic — after time/calorie)
				{Pattern: `(under|less than|cheaper than)\s+(\d+(?:\.\d+)?)`, Field: "price", Operator: "lt", Multiplier: 100},
				// Dietary / Nutritional tags
				{Pattern: `\b(low carb|low-carb)\b`, Field: "tags", Operator: "eq", Value: "Low Carb"},
				{Pattern: `\b(high protein|high-protein)\b`, Field: "tags", Operator: "eq", Value: "High Protein"},
				{Pattern: `\b(low calories?|low cal|light)\b`, Field: "total_calories", Operator: "lte", Value: "650", Strip: true},
				{Pattern: `\b(calorie smart|cal smart)\b`, Field: "tags", Operator: "eq", Value: "Calorie Smart", Strip: true},
				// Dietary tags with negation (all strip: true)
				{Pattern: `\b(no gluten|gluten[ -]?free)\b`, Field: "dietary", Operator: "eq", Value: "Gluten-Free Friendly", Strip: true},
				{Pattern: `\b(vegan)\b`, Field: "dietary", Operator: "eq", Value: "Vegan", Strip: true},
				{Pattern: `\b(vegetarian|no meat)\b`, Field: "dietary", Operator: "eq", Value: "Vegetarian", Strip: true},
				{Pattern: `\b(not spicy|no spicy|no spice)\b`, Field: "dietary", Operator: "eq", Value: "Non-spicy", Strip: true},
				{Pattern: `\b(no pork|pork[ -]?free)\b`, Field: "dietary", Operator: "eq", Value: "Pork-free", Strip: true},
				{Pattern: `\b(no dairy|dairy[ -]?free)\b`, Field: "dietary", Operator: "eq", Value: "Dairy-free", Strip: true},
				// Difficulty
				{Pattern: `\b(easy)\b`, Field: "difficulty_level", Operator: "eq", Value: "1"},
				// Servings
				{Pattern: `\b(for)\s+(\d+)\s*(people|persons?|servings?)\b`, Field: "servings", Operator: "eq"},
			},
			SortRules: []config.SortRule{
				{Pattern: `(cheapest|lowest price)`, Field: "price", Direction: "asc"},
				{Pattern: `(newest|most recent)`, Field: "created_at", Direction: "desc"},
			},
		},
		"de": {
			FilterRules: []config.FilterRule{
				{Pattern: `(unter|weniger als)\s+(\d+(?:\.\d+)?)`, Field: "price", Operator: "lt", Multiplier: 100},
				{Pattern: `\b(schnell)\b`, Field: "preparation_time", Operator: "lte", Value: "1800"},
			},
			SortRules: []config.SortRule{
				{Pattern: `(günstigste|billigste)`, Field: "price", Direction: "asc"},
			},
		},
	}

	return New(logger, nil,
		Normalizer{},
		Tokenizer{},
		NewSpellResolver(spellChecker, spellCfg, logger),
		NewSynonymExpander(linguisticLookup, logger),
		NewCompoundHandler(compoundLookup, logger),
		NewStopwordFilter(stopwords),
		NewConceptRecognizer(conceptSearcher, conceptCfg, logger),
		AmbiguityResolver{},
		NewComprehensionEngine(comprehensionCfg),
	)
}

func TestE2EGolden(t *testing.T) {
	pattern := filepath.Join("..", "..", "..", "testdata", "golden", "e2e_*.json")
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, files, "no e2e golden test files found matching %s", pattern)

	p := newE2EPipeline()

	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)

		var tc e2eGoldenTestCase
		require.NoError(t, json.Unmarshal(data, &tc))

		t.Run(tc.Name, func(t *testing.T) {
			state := &model.QueryState{
				OriginalQuery:   tc.Input.Query,
				NormalizedQuery: tc.Input.Query,
				Locale:          tc.Input.Locale,
				Market:          tc.Input.Market,
			}

			err := p.Run(context.Background(), state, false)
			require.NoError(t, err)

			resp := state.ToResponse()

			assert.Equal(t, tc.Expected.OriginalQuery, resp.OriginalQuery)
			assert.Equal(t, tc.Expected.NormalizedQuery, resp.NormalizedQuery)
			assert.Equal(t, tc.Expected.Tokens, resp.Tokens)
			assert.Equal(t, tc.Expected.Rewrites, resp.Rewrites)
			assert.Equal(t, tc.Expected.Concepts, resp.Concepts)
			assert.Equal(t, tc.Expected.Filters, resp.Filters)
			assert.Equal(t, tc.Expected.Sort, resp.Sort)
		})
	}
}
