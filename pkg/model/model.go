package model

import (
	"sort"
	"strings"
)

// AnalyzeRequest is the input contract for the /v1/analyze endpoint.
type AnalyzeRequest struct {
	Query          string   `json:"query"`
	Locale         string   `json:"locale"`
	Market         string   `json:"market"`
	ProcessorNames []string `json:"processorNames,omitempty"`
}

// AnalyzeResponse is the output contract for the /v1/analyze endpoint.
type AnalyzeResponse struct {
	OriginalQuery   string         `json:"originalQuery"`
	NormalizedQuery string         `json:"normalizedQuery"`
	Tokens          []Token        `json:"tokens"`
	Rewrites        []string       `json:"rewrites"`
	Concepts        []ConceptMatch `json:"concepts"`
	Filters         []Filter       `json:"filters"`
	Sort            *SortSpec      `json:"sort,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
}

// Token represents a single token in the query.
type Token struct {
	Value      string `json:"value"`
	Normalized string `json:"normalized"`
	Position   int    `json:"position"`
}

// ConceptMatch represents a recognized concept from the query.
type ConceptMatch struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	MatchedText string  `json:"matchedText"`
	Field       string  `json:"field,omitempty"`
	Score       float64 `json:"score"`
	Source      string  `json:"source"`
	Start       int     `json:"start"`
	End         int     `json:"end"`
}

// Filter represents an extracted filter directive.
type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// SortSpec represents an extracted sort directive.
type SortSpec struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// EdgeType represents the type of linguistic relationship.
type EdgeType string

const (
	EdgeExact    EdgeType = "exact"
	EdgeSynonym  EdgeType = "synonym"
	EdgeHypernym EdgeType = "hypernym"
	EdgeCompound EdgeType = "compound"
	EdgeSpell1   EdgeType = "spell1"
	EdgeSpell2   EdgeType = "spell2"
)

// BoostWeight returns the normalization weight for a given edge type.
func (e EdgeType) BoostWeight() float64 {
	switch e {
	case EdgeExact:
		return 1.0
	case EdgeSynonym:
		return 0.9
	case EdgeHypernym:
		return 0.8
	case EdgeCompound:
		return 0.8
	case EdgeSpell1:
		return 0.6
	case EdgeSpell2:
		return 0.3
	default:
		return 1.0
	}
}

// LinguisticEntry represents a synonym/hypernym entry from the linguistic index.
type LinguisticEntry struct {
	Term     string   `json:"term"`
	Type     EdgeType `json:"type"`
	Locale   string   `json:"locale"`
	Variants []string `json:"variants"`
}

// Shingle represents a contiguous span of tokens.
type Shingle struct {
	Text       string
	StartPos   int
	EndPos     int
	TokenCount int
}

// StepDebug holds debug info for a single pipeline step.
type StepDebug struct {
	Step    string      `json:"step"`
	Elapsed string      `json:"elapsed"`
	Output  interface{} `json:"output,omitempty"`
}

// QueryState is the mutable state passed through each pipeline step.
type QueryState struct {
	OriginalQuery   string
	NormalizedQuery string
	Locale          string
	Market          string
	Tokens          []Token
	Concepts        []ConceptMatch
	Filters         []Filter
	Sort            *SortSpec
	Warnings        []string
	Debug           []StepDebug
}

// ToResponse converts the pipeline state into the API response.
func (s *QueryState) ToResponse() AnalyzeResponse {
	rewrites := []string{}
	if s.NormalizedQuery != s.OriginalQuery {
		rewrites = append(rewrites, s.NormalizedQuery)
	}

	tokens := MergeConceptTokens(s.Tokens, s.Concepts)
	if tokens == nil {
		tokens = []Token{}
	}
	concepts := s.Concepts
	if concepts == nil {
		concepts = []ConceptMatch{}
	}
	filters := s.Filters
	if filters == nil {
		filters = []Filter{}
	}

	return AnalyzeResponse{
		OriginalQuery:   s.OriginalQuery,
		NormalizedQuery: s.NormalizedQuery,
		Tokens:          tokens,
		Rewrites:        rewrites,
		Concepts:        concepts,
		Filters:         filters,
		Sort:            s.Sort,
		Warnings:        s.Warnings,
	}
}

// mergeConceptTokens merges tokens that are covered by concept spans into
// compound tokens. For example, tokens ["high", "protein"] with a concept
// spanning positions 0–1 become ["high protein"].
func MergeConceptTokens(tokens []Token, concepts []ConceptMatch) []Token {
	if len(concepts) == 0 || len(tokens) == 0 {
		return tokens
	}

	// Build position-to-token index.
	byPos := make(map[int]int, len(tokens))
	for i, t := range tokens {
		byPos[t.Position] = i
	}

	// Sort concepts by start position.
	sorted := make([]ConceptMatch, len(concepts))
	copy(sorted, concepts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Start < sorted[j].Start
	})

	// Mark which token indices are consumed by a concept.
	consumed := make(map[int]bool)
	type mergedToken struct {
		token Token
		pos   int // original start position for ordering
	}
	var merged []mergedToken

	for _, c := range sorted {
		// Collect tokens in this concept's span.
		var values []string
		var normalized []string
		var indices []int
		for pos := c.Start; pos <= c.End; pos++ {
			if idx, ok := byPos[pos]; ok && !consumed[idx] {
				values = append(values, tokens[idx].Value)
				normalized = append(normalized, tokens[idx].Normalized)
				indices = append(indices, idx)
			}
		}
		if len(values) <= 1 {
			continue // single-token concept, nothing to merge
		}
		for _, idx := range indices {
			consumed[idx] = true
		}
		merged = append(merged, mergedToken{
			token: Token{
				Value:      strings.Join(values, " "),
				Normalized: strings.Join(normalized, " "),
				Position:   c.Start,
			},
			pos: c.Start,
		})
	}

	if len(merged) == 0 {
		return tokens
	}

	// Build final token list: unconsumed tokens + merged tokens, sorted by position.
	var result []Token
	for i, t := range tokens {
		if !consumed[i] {
			result = append(result, t)
		}
	}
	for _, m := range merged {
		result = append(result, m.token)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Position < result[j].Position
	})

	return result
}
