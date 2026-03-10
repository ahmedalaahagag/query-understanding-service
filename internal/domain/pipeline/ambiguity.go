package pipeline

import (
	"context"
	"sort"

	"github.com/hellofresh/qus/pkg/model"
)

// AmbiguityResolver is a pipeline step that removes overlapping concept matches,
// preferring longest span, then highest score, then earliest position.
type AmbiguityResolver struct{}

func (AmbiguityResolver) Name() string { return "ambiguity" }

func (AmbiguityResolver) Process(_ context.Context, state *model.QueryState) error {
	if len(state.Concepts) <= 1 {
		return nil
	}

	// Sort: longest span first, then highest score, then earliest start
	sort.Slice(state.Concepts, func(i, j int) bool {
		spanI := state.Concepts[i].End - state.Concepts[i].Start
		spanJ := state.Concepts[j].End - state.Concepts[j].Start
		if spanI != spanJ {
			return spanI > spanJ
		}
		if state.Concepts[i].Score != state.Concepts[j].Score {
			return state.Concepts[i].Score > state.Concepts[j].Score
		}
		return state.Concepts[i].Start < state.Concepts[j].Start
	})

	// Greedy non-overlapping selection
	var resolved []model.ConceptMatch
	taken := make(map[int]bool) // positions already claimed

	for _, concept := range state.Concepts {
		if overlaps(concept, taken) {
			continue
		}
		resolved = append(resolved, concept)
		for pos := concept.Start; pos <= concept.End; pos++ {
			taken[pos] = true
		}
	}

	state.Concepts = resolved
	return nil
}

func overlaps(c model.ConceptMatch, taken map[int]bool) bool {
	for pos := c.Start; pos <= c.End; pos++ {
		if taken[pos] {
			return true
		}
	}
	return false
}
