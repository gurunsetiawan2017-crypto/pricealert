package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/runtime/state"
	"github.com/pricealert/pricealert/internal/runtime/worker"
)

func TestActiveKeywordBecomesEligibleAndExecuted(t *testing.T) {
	clock := &stubClock{current: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	source := stubSource{
		keywords: []domain.TrackedKeyword{
			{ID: "kw_1", Status: domain.TrackedKeywordStatusActive, IntervalMinutes: 5},
		},
	}
	executor := newFakeExecutor()
	stateStore := state.NewStore()
	s := New(source, stateStore, worker.New(stateStore, executor, clock), clock)

	result, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if len(result.Started) != 1 || result.Started[0] != "kw_1" {
		t.Fatalf("started = %v, want [kw_1]", result.Started)
	}

	executor.waitForCalls(t, 1)
}

func TestPausedKeywordIsSkipped(t *testing.T) {
	clock := &stubClock{current: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	source := stubSource{
		keywords: []domain.TrackedKeyword{
			{ID: "kw_1", Status: domain.TrackedKeywordStatusPaused, IntervalMinutes: 5},
		},
	}
	executor := newFakeExecutor()
	stateStore := state.NewStore()
	s := New(source, stateStore, worker.New(stateStore, executor, clock), clock)

	result, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if len(result.Started) != 0 {
		t.Fatalf("started = %v, want empty", result.Started)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "kw_1" {
		t.Fatalf("skipped = %v, want [kw_1]", result.Skipped)
	}
	if executor.callCount() != 0 {
		t.Fatalf("executor calls = %d, want 0", executor.callCount())
	}
}

func TestOverlappingScanForSameKeywordIsBlocked(t *testing.T) {
	clock := &stubClock{current: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	source := stubSource{
		keywords: []domain.TrackedKeyword{
			{ID: "kw_1", Status: domain.TrackedKeywordStatusActive, IntervalMinutes: 5},
		},
	}
	executor := newFakeExecutor()
	executor.blockKeyword("kw_1")
	stateStore := state.NewStore()
	s := New(source, stateStore, worker.New(stateStore, executor, clock), clock)

	first, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(first.Started) != 1 {
		t.Fatalf("first started = %v, want 1 item", first.Started)
	}
	executor.waitForCalls(t, 1)

	second, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() second error = %v", err)
	}
	if len(second.Started) != 0 {
		t.Fatalf("second started = %v, want empty", second.Started)
	}
	if executor.callCount() != 1 {
		t.Fatalf("executor calls = %d, want 1", executor.callCount())
	}

	executor.releaseKeyword("kw_1")
	executor.waitForCompletions(t, 1)
}

func TestNextEligibleRunIsBasedOnCompletionTime(t *testing.T) {
	clock := &stubClock{current: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	source := stubSource{
		keywords: []domain.TrackedKeyword{
			{ID: "kw_1", Status: domain.TrackedKeywordStatusActive, IntervalMinutes: 5},
		},
	}
	executor := newFakeExecutor()
	stateStore := state.NewStore()
	s := New(source, stateStore, worker.New(stateStore, executor, clock), clock)

	_, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	executor.waitForCompletions(t, 1)

	clock.current = time.Date(2026, 4, 2, 10, 4, 0, 0, time.UTC)
	resultEarly, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() early error = %v", err)
	}
	if len(resultEarly.Started) != 0 {
		t.Fatalf("early started = %v, want empty", resultEarly.Started)
	}

	clock.current = time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)
	resultOnTime, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() on-time error = %v", err)
	}
	if len(resultOnTime.Started) != 1 || resultOnTime.Started[0] != "kw_1" {
		t.Fatalf("on-time started = %v, want [kw_1]", resultOnTime.Started)
	}
}

func TestFailureOfOneScanDoesNotCrashSchedulingOthers(t *testing.T) {
	clock := &stubClock{current: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	source := stubSource{
		keywords: []domain.TrackedKeyword{
			{ID: "kw_1", Status: domain.TrackedKeywordStatusActive, IntervalMinutes: 5},
			{ID: "kw_2", Status: domain.TrackedKeywordStatusActive, IntervalMinutes: 5},
		},
	}
	executor := newFakeExecutor()
	executor.failKeyword("kw_1", errors.New("scan failed"))
	stateStore := state.NewStore()
	s := New(source, stateStore, worker.New(stateStore, executor, clock), clock)

	result, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(result.Started) != 2 {
		t.Fatalf("started = %v, want 2 items", result.Started)
	}

	executor.waitForCompletions(t, 2)
	if executor.callCount() != 2 {
		t.Fatalf("executor calls = %d, want 2", executor.callCount())
	}
}

type stubSource struct {
	keywords []domain.TrackedKeyword
	err      error
}

func (s stubSource) ListKeywords(_ context.Context) ([]domain.TrackedKeyword, error) {
	if s.err != nil {
		return nil, s.err
	}

	result := make([]domain.TrackedKeyword, len(s.keywords))
	copy(result, s.keywords)
	return result, nil
}

type stubClock struct {
	current time.Time
}

func (s *stubClock) Now() time.Time {
	return s.current
}

type fakeExecutor struct {
	mu         sync.Mutex
	calls      []string
	callCh     chan struct{}
	completeCh chan struct{}
	blocked    map[string]chan struct{}
	failures   map[string]error
}

func newFakeExecutor() *fakeExecutor {
	return &fakeExecutor{
		callCh:     make(chan struct{}, 16),
		completeCh: make(chan struct{}, 16),
		blocked:    make(map[string]chan struct{}),
		failures:   make(map[string]error),
	}
}

func (f *fakeExecutor) Execute(_ context.Context, keyword domain.TrackedKeyword) error {
	f.mu.Lock()
	f.calls = append(f.calls, keyword.ID)
	block := f.blocked[keyword.ID]
	err := f.failures[keyword.ID]
	f.mu.Unlock()

	f.callCh <- struct{}{}
	if block != nil {
		<-block
	}
	f.completeCh <- struct{}{}
	return err
}

func (f *fakeExecutor) blockKeyword(keywordID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocked[keywordID] = make(chan struct{})
}

func (f *fakeExecutor) releaseKeyword(keywordID string) {
	f.mu.Lock()
	ch := f.blocked[keywordID]
	delete(f.blocked, keywordID)
	f.mu.Unlock()

	if ch != nil {
		close(ch)
	}
}

func (f *fakeExecutor) failKeyword(keywordID string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures[keywordID] = err
}

func (f *fakeExecutor) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeExecutor) waitForCalls(t *testing.T, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		select {
		case <-f.callCh:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for call %d", i+1)
		}
	}
}

func (f *fakeExecutor) waitForCompletions(t *testing.T, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		select {
		case <-f.completeCh:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for completion %d", i+1)
		}
	}
}
