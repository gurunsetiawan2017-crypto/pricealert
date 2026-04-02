package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/runtime/state"
)

func TestWorkerRespectsGlobalConcurrencyCap(t *testing.T) {
	clock := fakeClock{now: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	stateStore := state.NewStore()
	executor := newBlockingExecutor()
	worker := New(stateStore, executor, clock, 1)

	if !worker.Start(context.Background(), domain.TrackedKeyword{ID: "kw_1", IntervalMinutes: 5}) {
		t.Fatalf("expected first start to succeed")
	}
	executor.waitStarted(t, "kw_1")
	if worker.Start(context.Background(), domain.TrackedKeyword{ID: "kw_2", IntervalMinutes: 5}) {
		t.Fatalf("expected second start to be blocked by capacity")
	}

	executor.release("kw_1")
	if err := worker.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

func TestWorkerCloseStopsAcceptingAndWaitsForInflight(t *testing.T) {
	clock := fakeClock{now: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	stateStore := state.NewStore()
	executor := newBlockingExecutor()
	worker := New(stateStore, executor, clock, 1)

	if !worker.Start(context.Background(), domain.TrackedKeyword{ID: "kw_1", IntervalMinutes: 5}) {
		t.Fatalf("expected start to succeed")
	}
	executor.waitStarted(t, "kw_1")

	worker.StopAcceptingNewWork()
	if worker.Start(context.Background(), domain.TrackedKeyword{ID: "kw_2", IntervalMinutes: 5}) {
		t.Fatalf("expected start to be rejected after shutdown")
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- worker.Wait(context.Background())
	}()

	select {
	case <-waitDone:
		t.Fatalf("wait returned before inflight work completed")
	case <-time.After(20 * time.Millisecond):
	}

	executor.release("kw_1")
	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("wait did not finish after release")
	}
}

func TestWorkerDoesNotAcquireSlotAfterShutdownBegins(t *testing.T) {
	clock := fakeClock{now: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	stateStore := state.NewStore()
	executor := newBlockingExecutor()
	worker := New(stateStore, executor, clock, 1)

	worker.StopAcceptingNewWork()
	if worker.Start(context.Background(), domain.TrackedKeyword{ID: "kw_1", IntervalMinutes: 5}) {
		t.Fatalf("expected start to be rejected after shutdown")
	}
	if status := worker.Status(); status.RunningCount != 0 {
		t.Fatalf("running count = %d, want 0", status.RunningCount)
	}
}

func TestWorkerRecordsFailureAndPanicAsLastError(t *testing.T) {
	clock := fakeClock{now: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	stateStore := state.NewStore()
	executor := &recordingExecutor{errByKeyword: map[string]error{
		"kw_1": errors.New("scan failed"),
		"kw_2": errPanic,
	}}
	worker := New(stateStore, executor, clock, 2)

	if !worker.Start(context.Background(), domain.TrackedKeyword{ID: "kw_1", IntervalMinutes: 5}) {
		t.Fatalf("expected kw_1 start")
	}
	if !worker.Start(context.Background(), domain.TrackedKeyword{ID: "kw_2", IntervalMinutes: 5}) {
		t.Fatalf("expected kw_2 start")
	}
	if err := worker.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	first := stateStore.Snapshot("kw_1")
	if first.LastError == nil || *first.LastError != "scan failed" {
		t.Fatalf("kw_1 last error = %#v", first.LastError)
	}

	second := stateStore.Snapshot("kw_2")
	if second.LastError == nil || *second.LastError == "" {
		t.Fatalf("kw_2 last error = %#v", second.LastError)
	}
}

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}

type blockingExecutor struct {
	mu               sync.Mutex
	releaseByKeyword map[string]chan struct{}
	startedByKeyword map[string]chan struct{}
}

func newBlockingExecutor() *blockingExecutor {
	return &blockingExecutor{
		releaseByKeyword: map[string]chan struct{}{},
		startedByKeyword: map[string]chan struct{}{},
	}
}

func (b *blockingExecutor) Execute(_ context.Context, keyword domain.TrackedKeyword) error {
	ch := make(chan struct{})
	started := make(chan struct{})
	b.mu.Lock()
	b.releaseByKeyword[keyword.ID] = ch
	b.startedByKeyword[keyword.ID] = started
	b.mu.Unlock()
	close(started)
	<-ch
	return nil
}

func (b *blockingExecutor) waitStarted(t *testing.T, keywordID string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		b.mu.Lock()
		ch := b.startedByKeyword[keywordID]
		b.mu.Unlock()
		if ch != nil {
			select {
			case <-ch:
				return
			case <-deadline:
				t.Fatalf("timed out waiting for %s to start", keywordID)
			}
		}

		select {
		case <-time.After(time.Millisecond):
		case <-deadline:
			t.Fatalf("timed out waiting for %s start channel", keywordID)
		}
	}
}

func (b *blockingExecutor) release(keywordID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.releaseByKeyword[keywordID]; ok {
		close(ch)
		delete(b.releaseByKeyword, keywordID)
	}
	delete(b.startedByKeyword, keywordID)
}

var errPanic = errors.New("panic")

type recordingExecutor struct {
	errByKeyword map[string]error
}

func (r *recordingExecutor) Execute(_ context.Context, keyword domain.TrackedKeyword) error {
	if err, ok := r.errByKeyword[keyword.ID]; ok {
		if errors.Is(err, errPanic) {
			panic("boom")
		}
		return err
	}
	return nil
}
