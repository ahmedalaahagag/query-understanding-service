package pipeline

import (
	"strings"

	"github.com/hellofresh/qus/internal/domain/model"
)

// GenerateShingles produces n-grams of size 1..maxSize from the token list.
// Results are ordered longest-first to support greedy matching.
func GenerateShingles(tokens []model.Token, maxSize int) []model.Shingle {
	if maxSize <= 0 || len(tokens) == 0 {
		return nil
	}
	if maxSize > len(tokens) {
		maxSize = len(tokens)
	}

	var shingles []model.Shingle

	// Longest first for greedy matching
	for size := maxSize; size >= 1; size-- {
		for start := 0; start <= len(tokens)-size; start++ {
			end := start + size - 1

			parts := make([]string, size)
			for i := 0; i < size; i++ {
				parts[i] = tokens[start+i].Normalized
			}

			shingles = append(shingles, model.Shingle{
				Text:       strings.Join(parts, " "),
				StartPos:   tokens[start].Position,
				EndPos:     tokens[end].Position,
				TokenCount: size,
			})
		}
	}

	return shingles
}
