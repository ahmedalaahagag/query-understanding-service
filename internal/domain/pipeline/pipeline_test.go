package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/hellofresh/qus/pkg/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStep struct {
	name string
	fn   func(ctx context.Context, state *model.QueryState) error
}

func (m *mockStep) Name() string { return m.name }
func (m *mockStep) Process(ctx context.Context, state *model.QueryState) error {
	return m.fn(ctx, state)
}

func TestPipeline_RunEmpty(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	p := New(logger, nil)

	state := &model.QueryState{OriginalQuery: "test"}
	err := p.Run(context.Background(), state, false)
	require.NoError(t, err)
}

func TestPipeline_RunSteps(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var order []string

	step1 := &mockStep{name: "step1", fn: func(_ context.Context, s *model.QueryState) error {
		order = append(order, "step1")
		s.NormalizedQuery = "modified"
		return nil
	}}

	step2 := &mockStep{name: "step2", fn: func(_ context.Context, s *model.QueryState) error {
		order = append(order, "step2")
		return nil
	}}

	p := New(logger, nil, step1, step2)

	state := &model.QueryState{OriginalQuery: "test"}
	err := p.Run(context.Background(), state, false)
	require.NoError(t, err)

	assert.Equal(t, []string{"step1", "step2"}, order)
	assert.Equal(t, "modified", state.NormalizedQuery)
}

func TestPipeline_StepError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	failing := &mockStep{name: "failing", fn: func(_ context.Context, s *model.QueryState) error {
		return errors.New("boom")
	}}

	p := New(logger, nil, failing)

	state := &model.QueryState{OriginalQuery: "test"}
	err := p.Run(context.Background(), state, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "step failing")
}

func TestPipeline_Debug(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	step := &mockStep{name: "noop", fn: func(_ context.Context, s *model.QueryState) error {
		return nil
	}}

	p := New(logger, nil, step)

	state := &model.QueryState{OriginalQuery: "test"}
	err := p.Run(context.Background(), state, true)
	require.NoError(t, err)

	require.Len(t, state.Debug, 1)
	assert.Equal(t, "noop", state.Debug[0].Step)
}
