package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/config"
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
	worker := rtworker.New(stateStore, fakeWorkerExecutor{}, clock, 1)
	runtime := &Runtime{
		scheduler: rtscheduler.New(
			stubKeywordSource{keywords: []domain.TrackedKeyword{}},
			stateStore,
			worker,
			clock,
		),
		worker: worker,
		state:  stateStore,
	}

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(result.Started) != 0 || len(result.Skipped) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestNewRuntimeUsesInjectedKeywordRepository(t *testing.T) {
	repo := &fakeTrackedKeywordRepo{}
	runtime := newRuntime(minimalConfig(), appRepositories{trackedKeywords: repo})

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if repo.listActiveCalls != 1 {
		t.Fatalf("ListActive calls = %d, want %d", repo.listActiveCalls, 1)
	}
	if len(result.Started) != 0 || len(result.Skipped) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRuntimeStatusReflectsWorkerConfig(t *testing.T) {
	runtime := newRuntime(minimalConfig(), appRepositories{trackedKeywords: &fakeTrackedKeywordRepo{}})

	status := runtime.Status()
	if !status.AcceptingNewWork {
		t.Fatalf("expected runtime to accept new work")
	}
	if status.MaxConcurrent != 2 {
		t.Fatalf("max concurrent = %d, want 2", status.MaxConcurrent)
	}
}

func TestRuntimeCloseStopsNewWork(t *testing.T) {
	stateStore := rtstate.NewStore()
	clock := stubClock{current: time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}
	blocking := &blockingExecutor{}
	worker := rtworker.New(stateStore, blocking, clock, 1)
	runtime := &Runtime{
		scheduler: rtscheduler.New(stubKeywordSource{keywords: []domain.TrackedKeyword{}}, stateStore, worker, clock),
		worker:    worker,
		state:     stateStore,
	}

	if !worker.Start(context.Background(), domain.TrackedKeyword{ID: "kw_1", IntervalMinutes: 5}) {
		t.Fatalf("expected initial start")
	}
	blocking.waitStarted(t)

	done := make(chan error, 1)
	go func() {
		done <- runtime.Close(context.Background())
	}()

	select {
	case <-done:
		t.Fatalf("close returned before inflight work completed")
	case <-time.After(20 * time.Millisecond):
	}

	blocking.release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("close did not finish")
	}

	if runtime.Status().AcceptingNewWork {
		t.Fatalf("expected runtime to stop accepting new work")
	}
}

func TestReconcileAbandonedRunningScanJobsMarksRunningJobsFailed(t *testing.T) {
	clock := stubClock{current: time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)}
	repo := &fakeStartupScanJobRepo{
		running: []domain.ScanJob{
			{ID: "scan_1", Status: domain.ScanJobStatusRunning},
			{ID: "scan_2", Status: domain.ScanJobStatusRunning},
		},
	}

	result, err := reconcileAbandonedRunningScanJobs(context.Background(), repo, clock)
	if err != nil {
		t.Fatalf("reconcileAbandonedRunningScanJobs() error = %v", err)
	}
	if repo.listRunningCalls != 1 {
		t.Fatalf("ListRunning calls = %d, want 1", repo.listRunningCalls)
	}
	if len(repo.markFailedIDs) != 2 {
		t.Fatalf("mark failed ids = %#v", repo.markFailedIDs)
	}
	if repo.markFailedReason != abandonedScanJobReason {
		t.Fatalf("mark failed reason = %q", repo.markFailedReason)
	}
	if result.ReconciledCount != 2 {
		t.Fatalf("reconciled count = %d, want 2", result.ReconciledCount)
	}
	if result.ReconciledAt == nil || !result.ReconciledAt.Equal(clock.current) {
		t.Fatalf("reconciled at = %#v", result.ReconciledAt)
	}
}

func TestReconcileAbandonedRunningScanJobsHandlesEmptySet(t *testing.T) {
	clock := stubClock{current: time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)}
	repo := &fakeStartupScanJobRepo{}

	result, err := reconcileAbandonedRunningScanJobs(context.Background(), repo, clock)
	if err != nil {
		t.Fatalf("reconcileAbandonedRunningScanJobs() error = %v", err)
	}
	if len(repo.markFailedIDs) != 0 {
		t.Fatalf("unexpected mark failed ids = %#v", repo.markFailedIDs)
	}
	if result.ReconciledCount != 0 {
		t.Fatalf("reconciled count = %d, want 0", result.ReconciledCount)
	}
}

func TestRuntimeStatusIncludesStartupReconciliation(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)
	runtime := newRuntime(minimalConfig(), appRepositories{trackedKeywords: &fakeTrackedKeywordRepo{}})
	runtime.startup = StartupReconciliationResult{
		ReconciledCount: 3,
		ReconciledAt:    &now,
	}

	status := runtime.Status()
	if status.ReconciledRunningJobs != 3 {
		t.Fatalf("reconciled running jobs = %d, want 3", status.ReconciledRunningJobs)
	}
	if status.LastReconciledAt == nil || !status.LastReconciledAt.Equal(now) {
		t.Fatalf("last reconciled at = %#v", status.LastReconciledAt)
	}
}

func TestPruneRawListingsUsesAgeCutoff(t *testing.T) {
	clock := stubClock{current: time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)}
	repo := &fakeStartupRawListingRepo{prunedCount: 7}

	result, err := pruneRawListings(context.Background(), repo, 48, clock)
	if err != nil {
		t.Fatalf("pruneRawListings() error = %v", err)
	}
	if repo.pruneCalls != 1 {
		t.Fatalf("prune calls = %d, want 1", repo.pruneCalls)
	}
	wantCutoff := clock.current.Add(-48 * time.Hour)
	if !repo.cutoff.Equal(wantCutoff) {
		t.Fatalf("cutoff = %v, want %v", repo.cutoff, wantCutoff)
	}
	if result.PrunedCount != 7 {
		t.Fatalf("pruned count = %d, want 7", result.PrunedCount)
	}
	if result.PrunedAt == nil || !result.PrunedAt.Equal(clock.current) {
		t.Fatalf("pruned at = %#v", result.PrunedAt)
	}
}

func TestPruneRawListingsCanBeDisabled(t *testing.T) {
	clock := stubClock{current: time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)}
	repo := &fakeStartupRawListingRepo{}

	result, err := pruneRawListings(context.Background(), repo, 0, clock)
	if err != nil {
		t.Fatalf("pruneRawListings() error = %v", err)
	}
	if repo.pruneCalls != 0 {
		t.Fatalf("prune calls = %d, want 0", repo.pruneCalls)
	}
	if result.PrunedCount != 0 || result.PrunedAt != nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestPruneAlertEventsUsesAgeCutoff(t *testing.T) {
	clock := stubClock{current: time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)}
	repo := &fakeStartupAlertEventRepo{prunedCount: 5}

	result, err := pruneAlertEvents(context.Background(), repo, 72, clock)
	if err != nil {
		t.Fatalf("pruneAlertEvents() error = %v", err)
	}
	wantCutoff := clock.current.Add(-72 * time.Hour)
	if !repo.cutoff.Equal(wantCutoff) {
		t.Fatalf("cutoff = %v, want %v", repo.cutoff, wantCutoff)
	}
	if result.PrunedCount != 5 {
		t.Fatalf("pruned count = %d, want 5", result.PrunedCount)
	}
}

func TestRuntimeStatusIncludesRawListingPruning(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)
	runtime := newRuntime(minimalConfig(), appRepositories{trackedKeywords: &fakeTrackedKeywordRepo{}})
	runtime.pruning = RawListingPruneResult{
		PrunedCount: 9,
		PrunedAt:    &now,
	}

	status := runtime.Status()
	if status.PrunedRawListings != 9 {
		t.Fatalf("pruned raw listings = %d, want 9", status.PrunedRawListings)
	}
	if status.LastPrunedAt == nil || !status.LastPrunedAt.Equal(now) {
		t.Fatalf("last pruned at = %#v", status.LastPrunedAt)
	}
}

func TestRuntimeStatusIncludesAlertPruning(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 5, 0, 0, time.UTC)
	runtime := newRuntime(minimalConfig(), appRepositories{trackedKeywords: &fakeTrackedKeywordRepo{}})
	runtime.alertPruning = AlertEventPruneResult{PrunedCount: 5, PrunedAt: &now}

	status := runtime.Status()
	if status.PrunedAlertEvents != 5 {
		t.Fatalf("pruned alert events = %d, want 5", status.PrunedAlertEvents)
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
func (f *fakeTrackedKeywordRepo) ListVisible(context.Context) ([]domain.TrackedKeyword, error) {
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

type blockingExecutor struct {
	mu      sync.Mutex
	done    chan struct{}
	started chan struct{}
}

func (b *blockingExecutor) Execute(context.Context, domain.TrackedKeyword) error {
	b.mu.Lock()
	if b.done == nil {
		b.done = make(chan struct{})
	}
	if b.started == nil {
		b.started = make(chan struct{})
	}
	started := b.started
	done := b.done
	b.mu.Unlock()

	close(started)
	<-done
	return nil
}

func (b *blockingExecutor) release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done != nil {
		close(b.done)
		b.done = nil
	}
}

func (b *blockingExecutor) waitStarted(t *testing.T) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		b.mu.Lock()
		started := b.started
		b.mu.Unlock()
		if started != nil {
			select {
			case <-started:
				return
			case <-deadline:
				t.Fatalf("timed out waiting for executor start")
			}
		}

		select {
		case <-time.After(time.Millisecond):
		case <-deadline:
			t.Fatalf("timed out waiting for executor start channel")
		}
	}
}

func minimalConfig() config.Config {
	return config.Config{
		Runtime: config.RuntimeConfig{
			MaxConcurrentScans: 2,
		},
		Scraper: config.ScraperConfig{
			TokopediaSearchEndpoint: "https://example.com/graphql",
			TimeoutSeconds:          5,
			RowsPerScan:             10,
		},
	}
}

type fakeStartupScanJobRepo struct {
	running          []domain.ScanJob
	listRunningCalls int
	markFailedIDs    []string
	markFailedReason string
	err              error
}

func (f *fakeStartupScanJobRepo) Create(context.Context, domain.ScanJob) error { return nil }
func (f *fakeStartupScanJobRepo) MarkSuccess(context.Context, string, int, int) error {
	return nil
}
func (f *fakeStartupScanJobRepo) MarkFailed(_ context.Context, id string, errorMessage string) error {
	if f.err != nil {
		return f.err
	}
	f.markFailedIDs = append(f.markFailedIDs, id)
	f.markFailedReason = errorMessage
	return nil
}
func (f *fakeStartupScanJobRepo) GetLatestByKeywordID(context.Context, string) (*domain.ScanJob, error) {
	return nil, nil
}
func (f *fakeStartupScanJobRepo) ListRunning(context.Context, int) ([]domain.ScanJob, error) {
	f.listRunningCalls++
	if f.err != nil {
		return nil, f.err
	}
	result := make([]domain.ScanJob, len(f.running))
	copy(result, f.running)
	return result, nil
}

type fakeStartupRawListingRepo struct {
	pruneCalls  int
	cutoff      time.Time
	prunedCount int
	err         error
}

func (f *fakeStartupRawListingRepo) CreateBatch(context.Context, []domain.RawListing) error {
	return nil
}
func (f *fakeStartupRawListingRepo) ListByScanJobID(context.Context, string) ([]domain.RawListing, error) {
	return nil, nil
}
func (f *fakeStartupRawListingRepo) PruneOlderThanScrapedAt(_ context.Context, cutoff time.Time) (int, error) {
	f.pruneCalls++
	f.cutoff = cutoff
	if f.err != nil {
		return 0, f.err
	}
	return f.prunedCount, nil
}

type fakeStartupAlertEventRepo struct {
	cutoff      time.Time
	prunedCount int
	err         error
}

func (f *fakeStartupAlertEventRepo) Create(context.Context, domain.AlertEvent) error  { return nil }
func (f *fakeStartupAlertEventRepo) MarkSentToTelegram(context.Context, string) error { return nil }
func (f *fakeStartupAlertEventRepo) ListRecentByKeywordID(context.Context, string, int) ([]domain.AlertEvent, error) {
	return nil, nil
}
func (f *fakeStartupAlertEventRepo) PruneOlderThanCreatedAt(_ context.Context, cutoff time.Time) (int, error) {
	f.cutoff = cutoff
	if f.err != nil {
		return 0, f.err
	}
	return f.prunedCount, nil
}
