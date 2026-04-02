package worker

import (
	"context"
	"fmt"
	"sync"
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
	state     *state.Store
	executor  Executor
	clock     Clock
	sem       chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
	accepting bool
}

func New(stateStore *state.Store, executor Executor, clock Clock, maxConcurrent int) *Worker {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	return &Worker{
		state:     stateStore,
		executor:  executor,
		clock:     clock,
		sem:       make(chan struct{}, maxConcurrent),
		accepting: true,
	}
}

func (w *Worker) Start(ctx context.Context, keyword domain.TrackedKeyword) bool {
	if !w.tryAcquireSlot() {
		return false
	}

	startedAt := w.clock.Now()
	if !w.state.MarkRunning(keyword.ID, startedAt) {
		w.releaseSlot()
		return false
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer w.releaseSlot()

		err := safelyExecute(ctx, w.executor, keyword)
		w.state.MarkFinished(keyword.ID, w.clock.Now(), keyword.IntervalMinutes, err)
	}()

	return true
}

func (w *Worker) Status() Status {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return Status{
		AcceptingNewWork: w.accepting,
		MaxConcurrent:    cap(w.sem),
		RunningCount:     len(w.sem),
	}
}

type Status struct {
	AcceptingNewWork bool
	MaxConcurrent    int
	RunningCount     int
}

func (w *Worker) StopAcceptingNewWork() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.accepting = false
}

func (w *Worker) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (w *Worker) tryAcquireSlot() bool {
	w.mu.RLock()
	accepting := w.accepting
	w.mu.RUnlock()
	if !accepting {
		return false
	}

	select {
	case w.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (w *Worker) releaseSlot() {
	select {
	case <-w.sem:
	default:
	}
}

func safelyExecute(ctx context.Context, executor Executor, keyword domain.TrackedKeyword) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("worker panic for keyword %s: %v", keyword.ID, recovered)
		}
	}()

	return executor.Execute(ctx, keyword)
}
