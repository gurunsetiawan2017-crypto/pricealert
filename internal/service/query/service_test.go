package query

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/dto"
)

func TestDashboardStateBuildsSelectedKeywordView(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)
	service := NewService(
		&fakeTrackedKeywordRepo{
			visible: []domain.TrackedKeyword{
				{ID: "kw_1", Keyword: "minyak goreng 2L", Status: domain.TrackedKeywordStatusActive},
				{ID: "kw_2", Keyword: "gula pasir 1kg", Status: domain.TrackedKeywordStatusPaused},
			},
		},
		&fakeGroupedListingRepo{
			byScanJobID: map[string][]domain.GroupedListing{
				"scan_1": {
					{ID: "grp_1", GroupKey: "g1", RepresentativeTitle: "Minyak Promo", RepresentativeSeller: "Seller A", BestPrice: 23800, ListingCount: 3, SampleURL: "https://example.com/1"},
				},
			},
		},
		&fakeSnapshotRepo{
			byKeywordID: map[string]*domain.MarketSnapshot{
				"kw_1": {ID: "snap_1", TrackedKeywordID: "kw_1", ScanJobID: "scan_1", GroupedCount: 1, RawCount: 3, Signal: domain.MarketSignalBuyNow, SnapshotAt: now},
			},
		},
		&fakePricePointRepo{},
		&fakeAlertEventRepo{
			byKeywordID: map[string][]domain.AlertEvent{
				"kw_1": {
					{ID: "evt_1", TrackedKeywordID: "kw_1", Level: domain.AlertLevelAlert, EventType: domain.AlertEventTypeNewLowest, Message: "new low", CreatedAt: now},
				},
				"kw_2": {
					{ID: "evt_2", TrackedKeywordID: "kw_2", Level: domain.AlertLevelInfo, EventType: domain.AlertEventTypeScanFailed, Message: "failed", CreatedAt: now},
				},
			},
		},
		nil,
	)

	selectedID := "kw_1"
	state, err := service.DashboardState(context.Background(), &selectedID)
	if err != nil {
		t.Fatalf("DashboardState() error = %v", err)
	}

	if len(state.TrackedKeywords) != 2 {
		t.Fatalf("tracked keywords = %d, want 2", len(state.TrackedKeywords))
	}
	if state.SelectedKeywordID == nil || *state.SelectedKeywordID != "kw_1" {
		t.Fatalf("selected keyword id = %v", state.SelectedKeywordID)
	}
	if state.SelectedSnapshot == nil || state.SelectedSnapshot.ID != "snap_1" {
		t.Fatalf("selected snapshot = %#v", state.SelectedSnapshot)
	}
	if len(state.TopDeals) != 1 || state.TopDeals[0].ID != "grp_1" {
		t.Fatalf("top deals = %#v", state.TopDeals)
	}
	if !state.TrackedKeywords[0].HasNewAlert {
		t.Fatalf("expected first keyword to have new alert")
	}
	if state.TrackedKeywords[1].HasNewAlert {
		t.Fatalf("expected second keyword to not have new alert")
	}
}

func TestDashboardStateFallsBackToFirstVisibleKeyword(t *testing.T) {
	service := NewService(
		&fakeTrackedKeywordRepo{
			visible: []domain.TrackedKeyword{
				{ID: "kw_1", Keyword: "minyak goreng 2L", Status: domain.TrackedKeywordStatusActive},
			},
		},
		&fakeGroupedListingRepo{},
		&fakeSnapshotRepo{},
		&fakePricePointRepo{},
		&fakeAlertEventRepo{},
		nil,
	)

	selectedID := "missing"
	state, err := service.DashboardState(context.Background(), &selectedID)
	if err != nil {
		t.Fatalf("DashboardState() error = %v", err)
	}
	if state.SelectedKeywordID == nil || *state.SelectedKeywordID != "kw_1" {
		t.Fatalf("selected keyword id = %v, want kw_1", state.SelectedKeywordID)
	}
}

func TestKeywordDetailBuildsCompleteDTO(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)
	minPrice := int64(23800)
	service := NewService(
		&fakeTrackedKeywordRepo{
			byID: map[string]*domain.TrackedKeyword{
				"kw_1": {
					ID:              "kw_1",
					Keyword:         "minyak goreng 2L",
					IntervalMinutes: 5,
					TelegramEnabled: true,
					Status:          domain.TrackedKeywordStatusActive,
				},
			},
		},
		&fakeGroupedListingRepo{
			byScanJobID: map[string][]domain.GroupedListing{
				"scan_1": {
					{ID: "grp_1", GroupKey: "g1", RepresentativeTitle: "Minyak Promo", RepresentativeSeller: "Seller A", BestPrice: minPrice, ListingCount: 3, SampleURL: "https://example.com/1"},
				},
			},
		},
		&fakeSnapshotRepo{
			byKeywordID: map[string]*domain.MarketSnapshot{
				"kw_1": {ID: "snap_1", TrackedKeywordID: "kw_1", ScanJobID: "scan_1", MinPrice: &minPrice, GroupedCount: 1, RawCount: 3, Signal: domain.MarketSignalGoodDeal, SnapshotAt: now},
			},
		},
		&fakePricePointRepo{
			byKeywordID: map[string][]domain.PricePoint{
				"kw_1": {
					{ID: "pp_1", TrackedKeywordID: "kw_1", ScanJobID: "scan_1", MinPrice: &minPrice, RecordedAt: now},
				},
			},
		},
		&fakeAlertEventRepo{
			byKeywordID: map[string][]domain.AlertEvent{
				"kw_1": {
					{ID: "evt_1", TrackedKeywordID: "kw_1", Level: domain.AlertLevelAlert, EventType: domain.AlertEventTypeThresholdHit, Message: "threshold", CreatedAt: now},
				},
			},
		},
		nil,
	)

	detail, err := service.KeywordDetail(context.Background(), "kw_1")
	if err != nil {
		t.Fatalf("KeywordDetail() error = %v", err)
	}

	if detail.Keyword.ID != "kw_1" {
		t.Fatalf("keyword id = %q", detail.Keyword.ID)
	}
	if detail.Snapshot == nil || detail.Snapshot.ID != "snap_1" {
		t.Fatalf("snapshot = %#v", detail.Snapshot)
	}
	if len(detail.TopDeals) != 1 || detail.TopDeals[0].ID != "grp_1" {
		t.Fatalf("top deals = %#v", detail.TopDeals)
	}
	if len(detail.RecentEvents) != 1 || detail.RecentEvents[0].ID != "evt_1" {
		t.Fatalf("recent events = %#v", detail.RecentEvents)
	}
	if len(detail.RecentHistory) != 1 || detail.RecentHistory[0].ID != "pp_1" {
		t.Fatalf("recent history = %#v", detail.RecentHistory)
	}
}

func TestKeywordDetailAllowsMissingSnapshot(t *testing.T) {
	service := NewService(
		&fakeTrackedKeywordRepo{
			byID: map[string]*domain.TrackedKeyword{
				"kw_1": {ID: "kw_1", Keyword: "minyak goreng 2L", Status: domain.TrackedKeywordStatusActive},
			},
		},
		&fakeGroupedListingRepo{},
		&fakeSnapshotRepo{err: sql.ErrNoRows},
		&fakePricePointRepo{},
		&fakeAlertEventRepo{},
		nil,
	)

	detail, err := service.KeywordDetail(context.Background(), "kw_1")
	if err != nil {
		t.Fatalf("KeywordDetail() error = %v", err)
	}
	if detail.Snapshot != nil {
		t.Fatalf("snapshot = %#v, want nil", detail.Snapshot)
	}
	if len(detail.TopDeals) != 0 {
		t.Fatalf("top deals = %#v, want empty", detail.TopDeals)
	}
}

type fakeTrackedKeywordRepo struct {
	visible []domain.TrackedKeyword
	byID    map[string]*domain.TrackedKeyword
}

func (f *fakeTrackedKeywordRepo) Create(context.Context, domain.TrackedKeyword) error { return nil }
func (f *fakeTrackedKeywordRepo) Update(context.Context, domain.TrackedKeyword) error { return nil }
func (f *fakeTrackedKeywordRepo) GetByID(_ context.Context, id string) (*domain.TrackedKeyword, error) {
	if keyword, ok := f.byID[id]; ok {
		return keyword, nil
	}
	return nil, sql.ErrNoRows
}
func (f *fakeTrackedKeywordRepo) ListActive(context.Context) ([]domain.TrackedKeyword, error) {
	return nil, nil
}
func (f *fakeTrackedKeywordRepo) ListVisible(context.Context) ([]domain.TrackedKeyword, error) {
	return f.visible, nil
}

type fakeGroupedListingRepo struct {
	byScanJobID map[string][]domain.GroupedListing
}

func (f *fakeGroupedListingRepo) CreateBatch(context.Context, []domain.GroupedListing) error {
	return nil
}
func (f *fakeGroupedListingRepo) ListByScanJobID(_ context.Context, scanJobID string) ([]domain.GroupedListing, error) {
	return f.byScanJobID[scanJobID], nil
}

type fakeSnapshotRepo struct {
	byKeywordID map[string]*domain.MarketSnapshot
	err         error
}

func (f *fakeSnapshotRepo) Create(context.Context, domain.MarketSnapshot) error { return nil }
func (f *fakeSnapshotRepo) GetLatestByKeywordID(_ context.Context, keywordID string) (*domain.MarketSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	if snapshot, ok := f.byKeywordID[keywordID]; ok {
		return snapshot, nil
	}
	return nil, sql.ErrNoRows
}

type fakePricePointRepo struct {
	byKeywordID map[string][]domain.PricePoint
}

func (f *fakePricePointRepo) Create(context.Context, domain.PricePoint) error { return nil }
func (f *fakePricePointRepo) ListRecentByKeywordID(_ context.Context, keywordID string, _ int) ([]domain.PricePoint, error) {
	return f.byKeywordID[keywordID], nil
}

type fakeAlertEventRepo struct {
	byKeywordID map[string][]domain.AlertEvent
	err         error
}

func (f *fakeAlertEventRepo) Create(context.Context, domain.AlertEvent) error  { return nil }
func (f *fakeAlertEventRepo) MarkSentToTelegram(context.Context, string) error { return nil }
func (f *fakeAlertEventRepo) ListRecentByKeywordID(_ context.Context, keywordID string, limit int) ([]domain.AlertEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	events := f.byKeywordID[keywordID]
	if limit > 0 && len(events) > limit {
		return events[:limit], nil
	}
	return events, nil
}
func (f *fakeAlertEventRepo) PruneOlderThanCreatedAt(context.Context, time.Time) (int, error) {
	return 0, nil
}

var _ repositoryTrackedKeywordRepo = (*fakeTrackedKeywordRepo)(nil)

type repositoryTrackedKeywordRepo interface {
	Create(context.Context, domain.TrackedKeyword) error
	Update(context.Context, domain.TrackedKeyword) error
	GetByID(context.Context, string) (*domain.TrackedKeyword, error)
	ListActive(context.Context) ([]domain.TrackedKeyword, error)
	ListVisible(context.Context) ([]domain.TrackedKeyword, error)
}

func TestChooseSelectedKeywordWithEmptyList(t *testing.T) {
	selected := chooseSelectedKeyword(nil, nil)
	if selected != nil {
		t.Fatalf("selected = %#v, want nil", selected)
	}
}

func TestDashboardStatePropagatesVisibleKeywordErrors(t *testing.T) {
	service := NewService(
		&errorTrackedKeywordRepo{err: errors.New("boom")},
		&fakeGroupedListingRepo{},
		&fakeSnapshotRepo{},
		&fakePricePointRepo{},
		&fakeAlertEventRepo{},
		nil,
	)

	_, err := service.DashboardState(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

type errorTrackedKeywordRepo struct {
	err error
}

func (e *errorTrackedKeywordRepo) Create(context.Context, domain.TrackedKeyword) error { return nil }
func (e *errorTrackedKeywordRepo) Update(context.Context, domain.TrackedKeyword) error { return nil }
func (e *errorTrackedKeywordRepo) GetByID(context.Context, string) (*domain.TrackedKeyword, error) {
	return nil, e.err
}
func (e *errorTrackedKeywordRepo) ListActive(context.Context) ([]domain.TrackedKeyword, error) {
	return nil, e.err
}
func (e *errorTrackedKeywordRepo) ListVisible(context.Context) ([]domain.TrackedKeyword, error) {
	return nil, e.err
}

func TestDashboardStateIncludesRuntimeStatusSummary(t *testing.T) {
	now := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)
	service := NewService(
		&fakeTrackedKeywordRepo{
			visible: []domain.TrackedKeyword{
				{ID: "kw_1", Keyword: "minyak goreng 2L", Status: domain.TrackedKeywordStatusActive},
			},
		},
		&fakeGroupedListingRepo{},
		&fakeSnapshotRepo{},
		&fakePricePointRepo{},
		&fakeAlertEventRepo{},
		fakeRuntimeStatusProvider{
			summary: &dto.RuntimeStatusSummary{
				AcceptingNewWork:      true,
				RunningCount:          1,
				MaxConcurrent:         2,
				ReconciledRunningJobs: 3,
				LastReconciledAt:      &now,
				PrunedRawListings:     9,
				LastPrunedAt:          &now,
			},
		},
	)

	state, err := service.DashboardState(context.Background(), nil)
	if err != nil {
		t.Fatalf("DashboardState() error = %v", err)
	}
	if state.RuntimeStatus == nil {
		t.Fatalf("runtime status = nil")
	}
	if state.RuntimeStatus.RunningCount != 1 || state.RuntimeStatus.MaxConcurrent != 2 {
		t.Fatalf("runtime status = %#v", state.RuntimeStatus)
	}
	if state.RuntimeStatus.PrunedRawListings != 9 {
		t.Fatalf("pruned raw listings = %d", state.RuntimeStatus.PrunedRawListings)
	}
}

type fakeRuntimeStatusProvider struct {
	summary *dto.RuntimeStatusSummary
	err     error
}

func (f fakeRuntimeStatusProvider) Summary(context.Context) (*dto.RuntimeStatusSummary, error) {
	return f.summary, f.err
}
