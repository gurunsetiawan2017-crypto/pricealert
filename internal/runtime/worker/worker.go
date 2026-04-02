package worker

import (
	"context"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/runtime/state"
)

type Executor interface {
	Execute(context.Context, domain.TrackedKeyword) error
}

type Clock interface {
	Now() time.Time
}

type Worker struct {
	state    *state.Store
	executor Executor
	clock    Clock
}

func New(stateStore *state.Store, executor Executor, clock Clock) *Worker {
	return &Worker{
		state:    stateStore,
		executor: executor,
		clock:    clock,
	}
}

func (w *Worker) Start(ctx context.Context, keyword domain.TrackedKeyword) bool {
	startedAt := w.clock.Now()
	if !w.state.MarkRunning(keyword.ID, startedAt) {
		return false
	}

	go func() {
		_ = w.executor.Execute(ctx, keyword)
		w.state.MarkFinished(keyword.ID, w.clock.Now(), keyword.IntervalMinutes)
	}()

	return true
}
