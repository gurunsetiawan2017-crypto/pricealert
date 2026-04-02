package worker

import (
	"context"
	"errors"
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

var (
	ErrNotAcceptingNewWork   = errors.New("worker is not accepting new work")
	ErrAtCapacity            = errors.New("worker is at capacity")
	ErrKeywordAlreadyRunning = errors.New("keyword scan is already running")
)

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

func (w *Worker) ExecuteNow(ctx context.Context, keyword domain.TrackedKeyword) error {
	acquired, acquireErr := w.tryAcquireSlotWithReason()
	if !acquired {
		return acquireErr
	}

	startedAt := w.clock.Now()
	if !w.state.MarkRunning(keyword.ID, startedAt) {
		w.releaseSlot()
		return ErrKeywordAlreadyRunning
	}

	defer w.releaseSlot()
	err := safelyExecute(ctx, w.executor, keyword)
	w.state.MarkFinished(keyword.ID, w.clock.Now(), keyword.IntervalMinutes, err)
	return err
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
	acquired, _ := w.tryAcquireSlotWithReason()
	return acquired
}

func (w *Worker) tryAcquireSlotWithReason() (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.accepting {
		return false, ErrNotAcceptingNewWork
	}
	select {
	case w.sem <- struct{}{}:
		return true, nil
	default:
		return false, ErrAtCapacity
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
