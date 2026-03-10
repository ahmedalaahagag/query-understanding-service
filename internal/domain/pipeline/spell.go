package pipeline

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/hellofresh/qus/pkg/model"
	"github.com/hellofresh/qus/internal/infra/opensearch"
	"github.com/hellofresh/qus/pkg/config"
	"github.com/sirupsen/logrus"
)

var skuPattern = regexp.MustCompile(`^[A-Za-z]{0,3}\d{3,}`)

// SpellResolver is a pipeline step that corrects misspelled tokens using OpenSearch suggest.
type SpellResolver struct {
	checker opensearch.SpellChecker
	cfg     config.SpellConfig
	logger  *logrus.Logger
}

// NewSpellResolver creates a new SpellResolver step.
func NewSpellResolver(checker opensearch.SpellChecker, cfg config.SpellConfig, logger *logrus.Logger) *SpellResolver {
	return &SpellResolver{
		checker: checker,
		cfg:     cfg,
		logger:  logger,
	}
}

func (s *SpellResolver) Name() string { return "spell" }

func (s *SpellResolver) Process(ctx context.Context, state *model.QueryState) error {
	if !s.cfg.Enabled || s.checker == nil {
		return nil
	}

	changed := false
	for i, tok := range state.Tokens {
		if s.shouldSkip(tok.Value) {
			continue
		}

		suggestions, err := s.checker.Suggest(ctx, tok.Value, state.Locale)
		if err != nil {
			s.logger.WithError(err).WithField("token", tok.Value).Warn("spell suggest failed, keeping original")
			state.Warnings = append(state.Warnings, "spell check failed for token: "+tok.Value)
			continue
		}

		if len(suggestions) == 0 {
			continue
		}

		best := suggestions[0]
		if best.Score < s.cfg.ConfidenceThreshold {
			continue
		}

		corrected := strings.ToLower(best.Text)
		if corrected != tok.Value {
			state.Tokens[i].Normalized = corrected
			changed = true
		}
	}

	if changed {
		s.rebuildNormalizedQuery(state)
	}

	return nil
}

func (s *SpellResolver) shouldSkip(token string) bool {
	// Skip short tokens
	if len(token) < s.cfg.MinTokenLength {
		return true
	}

	// Skip numeric tokens
	if isNumeric(token) {
		return true
	}

	// Skip likely SKU patterns (e.g. AB123, 12345)
	if skuPattern.MatchString(token) {
		return true
	}

	return false
}

func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) && r != '.' && r != ',' {
			return false
		}
	}
	return true
}

func (s *SpellResolver) rebuildNormalizedQuery(state *model.QueryState) {
	parts := make([]string, len(state.Tokens))
	for i, tok := range state.Tokens {
		parts[i] = tok.Normalized
	}
	state.NormalizedQuery = strings.Join(parts, " ")
}
