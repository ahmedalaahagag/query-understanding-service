package hybrid

import (
	"context"
	"fmt"
	"testing"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConceptSearcher struct {
	results map[string][]opensearch.ConceptHit
	err     error
}

func (m *mockConceptSearcher) SearchConcepts(_ context.Context, text, _, _ string) ([]opensearch.ConceptHit, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results[text], nil
}

func TestConceptResolver_ExactMatch(t *testing.T) {
	searcher := &mockConceptSearcher{
		results: map[string][]opensearch.ConceptHit{
			"burger": {
				{ID: "product_type_burger", Label: "burger", Field: "product_type", Score: 5.0, Source: "exact"},
			},
		},
	}

	resolver := NewConceptResolver(searcher, logrus.New())
	tokens := []model.Token{
		{Value: "chicken", Normalized: "chicken", Position: 0},
		{Value: "burger", Normalized: "burger", Position: 1},
	}
	candidates := []LLMCandidateConcept{
		{Label: "burger", Field: "product_type", Confidence: 0.94},
	}

	concepts, warnings := resolver.Resolve(context.Background(), candidates, tokens, "en-GB", "uk")
	require.Len(t, concepts, 1)
	assert.Equal(t, "product_type_burger", concepts[0].ID)
	assert.Equal(t, "burger", concepts[0].Label)
	assert.Equal(t, "llm", concepts[0].Source)
	assert.Equal(t, 1, concepts[0].Start)
	assert.Empty(t, warnings)
}

func TestConceptResolver_Unresolved(t *testing.T) {
	searcher := &mockConceptSearcher{
		results: map[string][]opensearch.ConceptHit{},
	}

	resolver := NewConceptResolver(searcher, logrus.New())
	candidates := []LLMCandidateConcept{
		{Label: "nonexistent", Field: "product_type", Confidence: 0.9},
	}

	concepts, warnings := resolver.Resolve(context.Background(), candidates, nil, "en-GB", "uk")
	assert.Empty(t, concepts)
	assert.Contains(t, warnings[0], "unresolved concept dropped")
}

func TestConceptResolver_SearchError(t *testing.T) {
	searcher := &mockConceptSearcher{
		err: fmt.Errorf("connection refused"),
	}

	resolver := NewConceptResolver(searcher, logrus.New())
	candidates := []LLMCandidateConcept{
		{Label: "burger", Field: "product_type", Confidence: 0.9},
	}

	concepts, warnings := resolver.Resolve(context.Background(), candidates, nil, "en-GB", "uk")
	assert.Empty(t, concepts)
	assert.Contains(t, warnings[0], "concept resolution failed")
}

func TestConceptResolver_NilSearcher(t *testing.T) {
	resolver := NewConceptResolver(nil, logrus.New())
	candidates := []LLMCandidateConcept{
		{Label: "burger", Field: "product_type", Confidence: 0.9},
	}

	concepts, warnings := resolver.Resolve(context.Background(), candidates, nil, "en-GB", "uk")
	assert.Nil(t, concepts)
	assert.Nil(t, warnings)
}

func TestConceptResolver_MultipleCandidates(t *testing.T) {
	searcher := &mockConceptSearcher{
		results: map[string][]opensearch.ConceptHit{
			"burger": {
				{ID: "product_type_burger", Label: "burger", Field: "product_type", Score: 5.0, Source: "exact"},
			},
			"chicken": {
				{ID: "ingredient_chicken", Label: "chicken", Field: "ingredient", Score: 4.0, Source: "exact"},
			},
		},
	}

	resolver := NewConceptResolver(searcher, logrus.New())
	tokens := []model.Token{
		{Value: "chicken", Normalized: "chicken", Position: 0},
		{Value: "burger", Normalized: "burger", Position: 1},
	}
	candidates := []LLMCandidateConcept{
		{Label: "burger", Field: "product_type", Confidence: 0.94},
		{Label: "chicken", Field: "ingredient", Confidence: 0.88},
	}

	concepts, warnings := resolver.Resolve(context.Background(), candidates, tokens, "en-GB", "uk")
	require.Len(t, concepts, 2)
	assert.Equal(t, "product_type_burger", concepts[0].ID)
	assert.Equal(t, "ingredient_chicken", concepts[1].ID)
	assert.Empty(t, warnings)
}
