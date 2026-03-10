package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/ahmedalaahagag/query-understanding-service/pkg/model"
	"github.com/ahmedalaahagag/query-understanding-service/internal/infra/observability"
	"github.com/sirupsen/logrus"
)

// Pipeline orchestrates the sequential execution of processing steps.
type Pipeline struct {
	steps   []Step
	logger  *logrus.Logger
	metrics *observability.Metrics
}

// New creates a new Pipeline with the given steps.
func New(logger *logrus.Logger, metrics *observability.Metrics, steps ...Step) *Pipeline {
	return &Pipeline{
		steps:   steps,
		logger:  logger,
		metrics: metrics,
	}
}

// Run executes all steps sequentially on the given state.
func (p *Pipeline) Run(ctx context.Context, state *model.QueryState, debug bool) error {
	pipelineStart := time.Now()

	for _, step := range p.steps {
		start := time.Now()

		if err := step.Process(ctx, state); err != nil {
			p.logger.WithFields(logrus.Fields{
				"step":  step.Name(),
				"query": state.OriginalQuery,
			}).WithError(err).Error("pipeline step failed")
			return fmt.Errorf("step %s: %w", step.Name(), err)
		}

		elapsed := time.Since(start)

		if p.metrics != nil {
			p.metrics.StepDuration.WithLabelValues(step.Name()).Observe(elapsed.Seconds())
		}

		if debug {
			state.Debug = append(state.Debug, model.StepDebug{
				Step:    step.Name(),
				Elapsed: elapsed.String(),
			})
		}
	}

	totalDuration := time.Since(pipelineStart)

	if p.metrics != nil {
		p.metrics.PipelineDuration.WithLabelValues().Observe(totalDuration.Seconds())
		p.metrics.ConceptsFound.Observe(float64(len(state.Concepts)))
	}

	p.logger.WithFields(logrus.Fields{
		"query":      state.OriginalQuery,
		"normalized": state.NormalizedQuery,
		"tokens":     len(state.Tokens),
		"concepts":   len(state.Concepts),
		"filters":    len(state.Filters),
		"has_sort":   state.Sort != nil,
		"warnings":   len(state.Warnings),
		"duration":   totalDuration.String(),
	}).Info("pipeline completed")

	return nil
}
