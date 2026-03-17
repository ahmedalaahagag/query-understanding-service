package pipeline

import (
	"context"
	"strings"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/opensearch"
	"github.com/ahmedalaahagag/query-understanding-service/pkg/config"
	"github.com/sirupsen/logrus"
)

// Source-based scoring for concept matches.
const (
	scoreExact    = 1.0
	scoreSynonym  = 0.9
	scoreCompound = 0.8
	scoreSpell    = 0.7
)

// ConceptRecognizer is a pipeline step that identifies concepts from token shingles
// by querying an OpenSearch concept index.
type ConceptRecognizer struct {
	searcher opensearch.ConceptSearcher
	cfg      config.ConceptConfig
	logger   *logrus.Logger
}

// NewConceptRecognizer creates a new ConceptRecognizer step.
func NewConceptRecognizer(searcher opensearch.ConceptSearcher, cfg config.ConceptConfig, logger *logrus.Logger) *ConceptRecognizer {
	if cfg.ShingleMaxSize <= 0 {
		cfg.ShingleMaxSize = 4
	}
	if cfg.MaxMatchesPerSpan <= 0 {
		cfg.MaxMatchesPerSpan = 3
	}
	return &ConceptRecognizer{
		searcher: searcher,
		cfg:      cfg,
		logger:   logger,
	}
}

func (c *ConceptRecognizer) Name() string { return "concept" }

func (c *ConceptRecognizer) Process(ctx context.Context, state *model.QueryState) error {
	if c.searcher == nil || len(state.Tokens) == 0 {
		return nil
	}

	shingles := GenerateShingles(state.Tokens, c.cfg.ShingleMaxSize)

	var concepts []model.ConceptMatch

	for _, shingle := range shingles {
		hits, err := c.searcher.SearchConcepts(ctx, shingle.Text, state.Locale, state.Market)
		if err != nil {
			c.logger.WithError(err).WithField("shingle", shingle.Text).Warn("concept search failed")
			continue
		}

		added := 0
		for _, hit := range hits {
			if added >= c.cfg.MaxMatchesPerSpan {
				break
			}

			// For multi-word shingles, only accept exact label matches.
			// OS cross_fields returns partial hits (e.g. "chicken sausage"
			// for "asian chicken" because "chicken" matches). These partial
			// hits would claim all shingle positions incorrectly. Each
			// token's concept will be found by its own 1-word shingle.
			if shingle.TokenCount > 1 && !strings.EqualFold(hit.Label, shingle.Text) {
				continue
			}

			score := scoreForSource(hit.Source)

			concepts = append(concepts, model.ConceptMatch{
				ID:          hit.ID,
				Label:       hit.Label,
				MatchedText: shingle.Text,
				Field:       hit.Field,
				Score:       score,
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

func scoreForSource(source string) float64 {
	switch source {
	case "exact":
		return scoreExact
	case "synonym":
		return scoreSynonym
	case "compound":
		return scoreCompound
	case "spell":
		return scoreSpell
	default:
		return scoreSpell
	}
}
