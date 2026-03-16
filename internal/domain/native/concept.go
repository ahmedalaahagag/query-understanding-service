package native

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/ahmedalaahagag/query-understanding-service/internal/domain/pipeline"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/sirupsen/logrus"
)

// NativeConceptRecognizer identifies concepts using OS fuzzy multi_match.
// Unlike v1, this handles synonyms and compound variations natively through
// OS fuzziness: AUTO + cross_fields matching — no YAML synonym/compound config needed.
type NativeConceptRecognizer struct {
	fuzzy  opensearch.FuzzySearcher
	cfg    config.ConceptConfig
	logger *logrus.Logger
}

func NewNativeConceptRecognizer(fuzzy opensearch.FuzzySearcher, cfg config.ConceptConfig, logger *logrus.Logger) *NativeConceptRecognizer {
	if cfg.ShingleMaxSize <= 0 {
		cfg.ShingleMaxSize = 4
	}
	if cfg.MaxMatchesPerSpan <= 0 {
		cfg.MaxMatchesPerSpan = 3
	}
	return &NativeConceptRecognizer{fuzzy: fuzzy, cfg: cfg, logger: logger}
}

func (n *NativeConceptRecognizer) Name() string { return "native_concept" }

func (n *NativeConceptRecognizer) Process(ctx context.Context, state *model.QueryState) error {
	if n.fuzzy == nil || len(state.Tokens) == 0 {
		return nil
	}

	shingles := pipeline.GenerateShingles(state.Tokens, n.cfg.ShingleMaxSize)

	var concepts []model.ConceptMatch
	for _, shingle := range shingles {
		hits, err := n.fuzzy.FuzzySearchConcepts(ctx, shingle.Text, state.Locale, state.Market)
		if err != nil {
			n.logger.WithError(err).WithField("shingle", shingle.Text).Debug("fuzzy concept search failed")
			continue
		}

		added := 0
		for _, hit := range hits {
			if added >= n.cfg.MaxMatchesPerSpan {
				break
			}
			// Reject fuzzy matches where the first letter differs.
			// Prevents false positives like "chick" → "ground chuck" (via "chuck" alias).
			if hit.Source == "fuzzy" && !firstLetterMatches(shingle.Text, hit.Label) {
				continue
			}
			concepts = append(concepts, model.ConceptMatch{
				ID:          hit.ID,
				Label:       hit.Label,
				MatchedText: shingle.Text,
				Field:       hit.Field,
				Score:       hit.Score,
				Source:      hit.Source,
				Start:       shingle.StartPos,
				End:         shingle.EndPos,
			})
			added++
		}
	}

	state.Concepts = concepts
	return nil
}

// firstLetterMatches checks if the first letter of the query matches the first
// letter of the concept label. For multi-word labels, it checks whether ANY word
// in the label starts with the same letter as the query, since fuzzy matches can
// hit on any word within the label/aliases.
func firstLetterMatches(query, label string) bool {
	qFirst, _ := utf8.DecodeRuneInString(strings.ToLower(query))
	if qFirst == utf8.RuneError {
		return false
	}
	for _, word := range strings.Fields(strings.ToLower(label)) {
		lFirst, _ := utf8.DecodeRuneInString(word)
		if lFirst == qFirst {
			return true
		}
	}
	return false
}
