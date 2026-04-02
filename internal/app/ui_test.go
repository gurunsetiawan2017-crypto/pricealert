package app

import (
	"context"
	"errors"
	"testing"
	"time"

	rtscheduler "github.com/pricealert/pricealert/internal/runtime/scheduler"
)

func TestRuntimeTriggerAdapterMapsRunResult(t *testing.T) {
	trigger := newRuntimeTrigger(fakeRuntimeRunner{
		result: rtscheduler.RunResult{
			Started: []string{"kw_1"},
			Skipped: []string{"kw_2"},
		},
	})

	result, err := trigger.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(result.Started) != 1 || result.Started[0] != "kw_1" {
		t.Fatalf("started = %#v", result.Started)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "kw_2" {
		t.Fatalf("skipped = %#v", result.Skipped)
	}
}

func TestRuntimeTriggerAdapterReturnsError(t *testing.T) {
	trigger := newRuntimeTrigger(fakeRuntimeRunner{err: errors.New("boom")})

	_, err := trigger.RunOnce(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestKeywordActionAdapterDelegates(t *testing.T) {
	service := &fakeKeywordActionService{}
	actions := newKeywordActions(service)

	if err := actions.AddKeyword(context.Background(), "minyak goreng 2L"); err != nil {
		t.Fatalf("AddKeyword() error = %v", err)
	}
	if err := actions.PauseKeyword(context.Background(), "kw_1"); err != nil {
		t.Fatalf("PauseKeyword() error = %v", err)
	}
	if err := actions.ResumeKeyword(context.Background(), "kw_1"); err != nil {
		t.Fatalf("ResumeKeyword() error = %v", err)
	}
	if err := actions.ArchiveKeyword(context.Background(), "kw_1"); err != nil {
		t.Fatalf("ArchiveKeyword() error = %v", err)
	}
	threshold := int64(25000)
	filter := "2L"
	if err := actions.UpdateThreshold(context.Background(), "kw_1", &threshold); err != nil {
		t.Fatalf("UpdateThreshold() error = %v", err)
	}
	if err := actions.UpdateInterval(context.Background(), "kw_1", 10); err != nil {
		t.Fatalf("UpdateInterval() error = %v", err)
	}
	if err := actions.UpdateBasicFilter(context.Background(), "kw_1", &filter); err != nil {
		t.Fatalf("UpdateBasicFilter() error = %v", err)
	}
	if err := actions.SetTelegramEnabled(context.Background(), "kw_1", true); err != nil {
		t.Fatalf("SetTelegramEnabled() error = %v", err)
	}

	if service.addedKeyword != "minyak goreng 2L" {
		t.Fatalf("added keyword = %q", service.addedKeyword)
	}
	if service.pausedKeywordID != "kw_1" || service.resumedKeywordID != "kw_1" || service.archivedKeywordID != "kw_1" {
		t.Fatalf("unexpected action ids: %#v", service)
	}
	if service.threshold == nil || *service.threshold != 25000 {
		t.Fatalf("threshold = %v", service.threshold)
	}
	if service.interval != 10 {
		t.Fatalf("interval = %d", service.interval)
	}
	if service.basicFilter == nil || *service.basicFilter != "2L" {
		t.Fatalf("basic filter = %v", service.basicFilter)
	}
	if !service.telegramEnabled {
		t.Fatalf("telegram enabled = false")
	}
}

func TestRuntimeStatusAdapterMapsSummary(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)
	adapter := newRuntimeStatusAdapter(fakeRuntimeStatusSource{
		status: RuntimeStatus{
			AcceptingNewWork:       true,
			RunningCount:           1,
			MaxConcurrent:          2,
			ReconciledRunningJobs:  3,
			LastReconciledAt:       &now,
			PrunedRawListings:      9,
			LastPrunedAt:           &now,
			PrunedAlertEvents:      5,
			LastAlertPrunedAt:      &now,
			PrunedPricePoints:      4,
			LastPricePointPrunedAt: &now,
		},
	})

	summary, err := adapter.Summary(context.Background())
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if summary == nil {
		t.Fatalf("summary = nil")
	}
	if summary.RunningCount != 1 || summary.MaxConcurrent != 2 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.PrunedRawListings != 9 {
		t.Fatalf("pruned raw listings = %d", summary.PrunedRawListings)
	}
	if summary.PrunedAlertEvents != 5 || summary.PrunedPricePoints != 4 {
		t.Fatalf("summary = %#v", summary)
	}
}

type fakeRuntimeRunner struct {
	result rtscheduler.RunResult
	err    error
}

func (f fakeRuntimeRunner) RunRuntimeOnce(context.Context) (rtscheduler.RunResult, error) {
	return f.result, f.err
}

type fakeKeywordActionService struct {
	addedKeyword      string
	pausedKeywordID   string
	resumedKeywordID  string
	archivedKeywordID string
	threshold         *int64
	interval          int
	basicFilter       *string
	telegramEnabled   bool
}

type fakeRuntimeStatusSource struct {
	status RuntimeStatus
}

func (f fakeRuntimeStatusSource) RuntimeStatus() RuntimeStatus {
	return f.status
}

func (f *fakeKeywordActionService) AddKeyword(_ context.Context, keyword string) error {
	f.addedKeyword = keyword
	return nil
}

func (f *fakeKeywordActionService) PauseKeyword(_ context.Context, keywordID string) error {
	f.pausedKeywordID = keywordID
	return nil
}

func (f *fakeKeywordActionService) ResumeKeyword(_ context.Context, keywordID string) error {
	f.resumedKeywordID = keywordID
	return nil
}

func (f *fakeKeywordActionService) ArchiveKeyword(_ context.Context, keywordID string) error {
	f.archivedKeywordID = keywordID
	return nil
}

func (f *fakeKeywordActionService) UpdateThreshold(_ context.Context, _ string, threshold *int64) error {
	f.threshold = threshold
	return nil
}

func (f *fakeKeywordActionService) UpdateInterval(_ context.Context, _ string, interval int) error {
	f.interval = interval
	return nil
}

func (f *fakeKeywordActionService) UpdateBasicFilter(_ context.Context, _ string, basicFilter *string) error {
	f.basicFilter = basicFilter
	return nil
}

func (f *fakeKeywordActionService) SetTelegramEnabled(_ context.Context, _ string, enabled bool) error {
	f.telegramEnabled = enabled
	return nil
}
