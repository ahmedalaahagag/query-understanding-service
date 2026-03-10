package pipeline

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/hellofresh/qus/internal/domain/model"
	"golang.org/x/text/unicode/norm"
)

// noisePunctuation matches punctuation that carries no search meaning.
// We keep hyphens (for compound words) and apostrophes (for contractions).
var noisePunctuation = regexp.MustCompile(`[^\w\s\-']`)

// collapseSpaces matches two or more consecutive whitespace characters.
var collapseSpaces = regexp.MustCompile(`\s{2,}`)

// Normalizer is a pipeline step that lowercases, trims, collapses whitespace,
// strips noise punctuation, and normalizes unicode.
type Normalizer struct{}

func (Normalizer) Name() string { return "normalize" }

func (Normalizer) Process(_ context.Context, state *model.QueryState) error {
	q := state.NormalizedQuery

	// Unicode NFC normalization — collapses composed characters
	q = norm.NFC.String(q)

	// Strip diacritics: decompose then drop combining marks
	q = stripDiacritics(q)

	// Lowercase
	q = strings.ToLower(q)

	// Remove noise punctuation (keep hyphens, apostrophes, alphanumerics, whitespace)
	q = noisePunctuation.ReplaceAllString(q, " ")

	// Collapse multiple spaces
	q = collapseSpaces.ReplaceAllString(q, " ")

	// Trim leading/trailing whitespace
	q = strings.TrimSpace(q)

	state.NormalizedQuery = q
	return nil
}

// stripDiacritics decomposes unicode and removes combining marks.
// e.g. "café" → "cafe", "über" → "uber"
func stripDiacritics(s string) string {
	// NFD decomposes characters: é → e + combining acute accent
	decomposed := norm.NFD.String(s)

	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if !unicode.Is(unicode.Mn, r) { // Mn = Mark, Nonspacing (combining marks)
			b.WriteRune(r)
		}
	}
	return b.String()
}
