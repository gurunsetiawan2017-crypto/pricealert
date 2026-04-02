package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	rtscheduler "github.com/pricealert/pricealert/internal/runtime/scheduler"
	rtstate "github.com/pricealert/pricealert/internal/runtime/state"
	rtworker "github.com/pricealert/pricealert/internal/runtime/worker"
	"github.com/pricealert/pricealert/internal/service/scan"
)

func TestTrackedKeywordSourceAdapterUsesListActive(t *testing.T) {
	repo := &fakeTrackedKeywordRepo{
		active: []domain.TrackedKeyword{
			{ID: "kw_1", Status: domain.TrackedKeywordStatusActive},
		},
	}

	got, err := trackedKeywordSourceAdapter{repo: repo}.ListKeywords(context.Background())
	if err != nil {
		t.Fatalf("ListKeywords() error = %v", err)
	}

	if repo.listActiveCalls != 1 {
		t.Fatalf("ListActive calls = %d, want %d", repo.listActiveCalls, 1)
	}
	if len(got) != 1 || got[0].ID != "kw_1" {
		t.Fatalf("keywords = %#v, want kw_1", got)
	}
}

func TestScanExecutorAdapterDelegatesToScanService(t *testing.T) {
	runner := &fakeScanRunner{}

	err := scanExecutorAdapter{scan: runner}.Execute(context.Background(), domain.TrackedKeyword{ID: "kw_1"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("scan service calls = %d, want %d", runner.calls, 1)
	}
}

func TestScanExecutorAdapterReturnsScanError(t *testing.T) {
	runner := &fakeScanRunner{err: errors.New("scan failed")}

	err := scanExecutorAdapter{scan: runner}.Execute(context.Background(), domain.TrackedKeyword{ID: "kw_1"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRuntimeRunOnceDelegatesToScheduler(t *testing.T) {
	clock := stubClock{current: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	stateStore := rtstate.NewStore()
	runtime := &Runtime{
		scheduler: rtscheduler.New(
			stubKeywordSource{keywords: []domain.TrackedKeyword{}},
			stateStore,
			rtworker.New(stateStore, fakeWorkerExecutor{}, clock),
			clock,
		),
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(result.Started) != 0 || len(result.Skipped) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

type fakeTrackedKeywordRepo struct {
	active          []domain.TrackedKeyword
	listActiveCalls int
}

func (f *fakeTrackedKeywordRepo) Create(context.Context, domain.TrackedKeyword) error { return nil }
func (f *fakeTrackedKeywordRepo) Update(context.Context, domain.TrackedKeyword) error { return nil }
func (f *fakeTrackedKeywordRepo) GetByID(context.Context, string) (*domain.TrackedKeyword, error) {
	return nil, nil
}
func (f *fakeTrackedKeywordRepo) ListActive(context.Context) ([]domain.TrackedKeyword, error) {
	f.listActiveCalls++
	return f.active, nil
}

type fakeScanRunner struct {
	calls int
	err   error
}

func (f *fakeScanRunner) Execute(context.Context, domain.TrackedKeyword) (*scan.Result, error) {
	f.calls++
	return nil, f.err
}

type stubKeywordSource struct {
	keywords []domain.TrackedKeyword
}

func (s stubKeywordSource) ListKeywords(context.Context) ([]domain.TrackedKeyword, error) {
	return s.keywords, nil
}

type stubClock struct {
	current time.Time
}

func (s stubClock) Now() time.Time {
	return s.current
}

type fakeWorkerExecutor struct{}

func (fakeWorkerExecutor) Execute(context.Context, domain.TrackedKeyword) error {
	return nil
}
