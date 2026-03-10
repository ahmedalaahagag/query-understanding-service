package hybrid

// LLMParseResult is the structured output expected from the LLM semantic parser.
type LLMParseResult struct {
	NormalizedQuery   string                `json:"normalizedQuery"`
	Rewrites          []string              `json:"rewrites"`
	CandidateConcepts []LLMCandidateConcept `json:"candidateConcepts"`
	Filters           []LLMFilter           `json:"filters"`
	Sort              *LLMSort              `json:"sort,omitempty"`
	Confidence        float64               `json:"confidence"`
	Warnings          []string              `json:"warnings,omitempty"`
}

// LLMCandidateConcept is a concept proposed by the LLM before validation.
type LLMCandidateConcept struct {
	Label      string  `json:"label"`
	Field      string  `json:"field"`
	Confidence float64 `json:"confidence"`
}

// LLMFilter is a filter proposed by the LLM before validation.
type LLMFilter struct {
	Field      string      `json:"field"`
	Operator   string      `json:"operator"`
	Value      interface{} `json:"value"`
	Confidence float64     `json:"confidence"`
}

// LLMSort is a sort directive proposed by the LLM before validation.
type LLMSort struct {
	Field      string  `json:"field"`
	Direction  string  `json:"direction"`
	Confidence float64 `json:"confidence"`
}
