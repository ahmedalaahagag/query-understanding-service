package pipeline

import (
	"context"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
)

// Step is a single processing step in the query analysis pipeline.
type Step interface {
	Name() string
	Process(ctx context.Context, state *model.QueryState) error
}
