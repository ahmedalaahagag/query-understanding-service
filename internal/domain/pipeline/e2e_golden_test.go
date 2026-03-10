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

	comprehensionCfg := config.ComprehensionConfig{
		PriceRules: []config.PriceRule{
			{Pattern: `(under|less than|cheaper than)\s+(\d+(?:\.\d+)?)`, Field: "price", Operator: "lt"},
		},
		SortRules: []config.SortRule{
			{Pattern: `(cheapest|lowest price)`, Field: "price", Direction: "asc"},
			{Pattern: `(newest|most recent)`, Field: "created_at", Direction: "desc"},
		},
	}

	return New(logger, nil,
		Normalizer{},
		Tokenizer{},
		NewSpellResolver(spellChecker, spellCfg, logger),
		NewSynonymExpander(linguisticLookup, logger),
		NewCompoundHandler(compoundLookup, logger),
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
