package adaptive

import (
	"regexp"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

// ComplexityScore holds the result of evaluating a v3 pipeline output.
type ComplexityScore struct {
	TokenCoverage      float64 // fraction of tokens matched by concepts (0..1)
	ConceptCount       int     // number of resolved concepts
	AvgConceptScore    float64 // mean concept match score
	SpellCorrections   int     // number of tokens that were spell-corrected
	FilterCount        int     // number of extracted filters
	TokenCount         int     // total tokens after comprehension/stopword
	OriginalTokenCount int     // tokens in the original query (before v3 processing)
	Conversational     bool    // query looks like natural language
	Score              float64 // final complexity score (0 = simple, 1 = complex)
	Escalate           bool    // true if LLM should be used
}

// ScorerConfig holds tuning thresholds for the complexity scorer.
type ScorerConfig struct {
	// MaxUncoveredRatio: fraction of tokens not covered by concepts
	// before the query is considered complex. Default: 0.5
	MaxUncoveredRatio float64 `yaml:"max_uncovered_ratio"`

	// MinConceptScore: minimum average concept score to consider
	// the v3 result confident. Default: 0.7
	MinConceptScore float64 `yaml:"min_concept_score"`

	// MaxSpellCorrections: spell corrections above this suggest unclear intent.
	// Default: 2
	MaxSpellCorrections int `yaml:"max_spell_corrections"`

	// MinTokensForConversational: token count above which conversational
	// pattern detection kicks in. Default: 5
	MinTokensForConversational int `yaml:"min_tokens_for_conversational"`
}

// DefaultScorerConfig returns sensible defaults.
func DefaultScorerConfig() ScorerConfig {
	return ScorerConfig{
		MaxUncoveredRatio:          0.5,
		MinConceptScore:            0.7,
		MaxSpellCorrections:        2,
		MinTokensForConversational: 5,
	}
}

// ScorerConfigFromYAML converts the YAML config to a ScorerConfig,
// applying defaults for any zero values.
func ScorerConfigFromYAML(cfg config.AdaptiveConfig) ScorerConfig {
	defaults := DefaultScorerConfig()
	sc := ScorerConfig{
		MaxUncoveredRatio:          cfg.MaxUncoveredRatio,
		MinConceptScore:            cfg.MinConceptScore,
		MaxSpellCorrections:        cfg.MaxSpellCorrections,
		MinTokensForConversational: cfg.MinTokensForConversational,
	}
	if sc.MaxUncoveredRatio == 0 {
		sc.MaxUncoveredRatio = defaults.MaxUncoveredRatio
	}
	if sc.MinConceptScore == 0 {
		sc.MinConceptScore = defaults.MinConceptScore
	}
	if sc.MaxSpellCorrections == 0 {
		sc.MaxSpellCorrections = defaults.MaxSpellCorrections
	}
	if sc.MinTokensForConversational == 0 {
		sc.MinTokensForConversational = defaults.MinTokensForConversational
	}
	return sc
}

// conversationalPatterns matches natural language phrasing that the
// deterministic pipeline can't parse well.
var conversationalPatterns = regexp.MustCompile(
	`(?i)\b(show me|i want|something|looking for|give me|can you|anything|recommend)\b`,
)

// Score evaluates the complexity of a query based on v3 pipeline output.
func Score(resp model.AnalyzeResponse, originalQuery string, cfg ScorerConfig) ComplexityScore {
	originalWords := strings.Fields(strings.ToLower(strings.TrimSpace(originalQuery)))

	cs := ComplexityScore{
		TokenCount:         len(resp.Tokens),
		OriginalTokenCount: len(originalWords),
		ConceptCount:       len(resp.Concepts),
		FilterCount:        len(resp.Filters),
	}

	// Count spell corrections: tokens where original differs from normalized.
	cs.SpellCorrections = countSpellCorrections(resp.Tokens)

	// Token coverage: what fraction of tokens are covered by concepts?
	if cs.TokenCount > 0 && cs.ConceptCount > 0 {
		coveredPositions := make(map[int]bool)
		for _, c := range resp.Concepts {
			for pos := c.Start; pos <= c.End; pos++ {
				coveredPositions[pos] = true
			}
		}
		covered := 0
		for _, t := range resp.Tokens {
			if coveredPositions[t.Position] {
				covered++
			}
		}
		cs.TokenCoverage = float64(covered) / float64(cs.TokenCount)
	}

	// Average concept score.
	if cs.ConceptCount > 0 {
		total := 0.0
		for _, c := range resp.Concepts {
			total += c.Score
		}
		cs.AvgConceptScore = total / float64(cs.ConceptCount)
	}

	// Conversational detection on the original query (before v3 strips tokens).
	// Use original word count — v3 may strip most tokens as stopwords/comprehension,
	// making the post-processed count misleadingly low.
	if cs.OriginalTokenCount >= cfg.MinTokensForConversational {
		cs.Conversational = conversationalPatterns.MatchString(originalQuery)
	}

	// Compute final score (0 = simple, 1 = complex).
	var score float64

	// Uncovered tokens are the strongest signal (weight: 0.4).
	uncoveredRatio := 1.0 - cs.TokenCoverage
	if uncoveredRatio > cfg.MaxUncoveredRatio {
		score += 0.4
	} else {
		score += 0.4 * (uncoveredRatio / cfg.MaxUncoveredRatio)
	}

	// Low concept scores suggest fuzzy/uncertain matches (weight: 0.2).
	if cs.ConceptCount > 0 && cs.AvgConceptScore < cfg.MinConceptScore {
		score += 0.2
	}

	// Multiple spell corrections suggest unclear intent (weight: 0.2).
	if cs.SpellCorrections > cfg.MaxSpellCorrections {
		score += 0.2
	} else if cs.SpellCorrections > 0 {
		score += 0.2 * (float64(cs.SpellCorrections) / float64(cfg.MaxSpellCorrections))
	}

	// Conversational queries need semantic understanding (weight: 0.2).
	if cs.Conversational {
		score += 0.2
	}

	cs.Score = score

	// Escalate if score exceeds 0.5 OR specific hard triggers:
	// - Conversational phrasing (deterministic can't handle "show me something easy")
	// - Heavy token reduction: v3 stripped most of the original query (e.g. 6 words → 1 token),
	//   meaning most of the query was stopwords/comprehension noise to the deterministic pipeline.
	//   This is a strong signal that the query needs semantic understanding.
	// - 3+ original tokens with zero concepts and zero filters (v3 understood nothing)
	heavyReduction := cs.OriginalTokenCount >= 4 && cs.TokenCount <= 1
	cs.Escalate = score > 0.5 ||
		cs.Conversational ||
		heavyReduction ||
		(cs.OriginalTokenCount >= 3 && cs.ConceptCount == 0 && cs.FilterCount == 0)

	return cs
}

func countSpellCorrections(tokens []model.Token) int {
	count := 0
	for _, t := range tokens {
		if strings.ToLower(t.Value) != strings.ToLower(t.Normalized) {
			count++
		}
	}
	return count
}
