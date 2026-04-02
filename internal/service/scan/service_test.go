package scan

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/service/alert"
	"github.com/pricealert/pricealert/internal/service/grouping"
	"github.com/pricealert/pricealert/internal/service/history"
	"github.com/pricealert/pricealert/internal/service/snapshot"
)

func TestExecuteHappyPath(t *testing.T) {
	clock := &stubClock{
		times: []time.Time{
			time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 5, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 10, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 15, 0, time.UTC),
		},
	}
	ids := &stubIDs{values: []string{"scan_1", "raw_1", "raw_2", "grp_1", "snap_1", "pp_1", "evt_1"}}
	scanJobs := &fakeScanJobRepo{}
	rawRepo := &fakeRawListingRepo{}
	groupedRepo := &fakeGroupedListingRepo{}
	snapshotRepo := &fakeSnapshotRepo{}
	pricePointRepo := &fakePricePointRepo{}
	alertRepo := &fakeAlertEventRepo{
		recentEvents: []domain.AlertEvent{},
	}

	service := NewService(
		stubScraper{
			listings: []domain.RawListing{
				{Title: "Bimoli Minyak Goreng 2L Promo", SellerName: "Seller A", Price: 22000, URL: "https://example.com/1", Source: "tokopedia"},
				{Title: "Bimoli Minyak Goreng 2 Liter", SellerName: "Seller B", Price: 25000, URL: "https://example.com/2", Source: "tokopedia"},
			},
		},
		nil,
		ids,
		clock,
		scanJobs,
		rawRepo,
		groupedRepo,
		snapshotRepo,
		pricePointRepo,
		alertRepo,
		grouping.NewService(),
		snapshot.NewService(),
		history.NewService(),
		alert.NewServiceWithConfig(alert.Config{
			ThresholdCooldown:        time.Hour,
			NewLowestCooldown:        time.Hour,
			MeaningfulImprovementPct: 0.03,
			MinHistoryPoints:         1,
		}),
	)

	threshold := int64(23000)
	result, err := service.Execute(context.Background(), domain.TrackedKeyword{
		ID:             "kw_1",
		Keyword:        "minyak goreng 2L",
		ThresholdPrice: &threshold,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if scanJobs.created.Status != domain.ScanJobStatusRunning {
		t.Fatalf("created scan status = %q, want %q", scanJobs.created.Status, domain.ScanJobStatusRunning)
	}
	if scanJobs.markSuccessID != "scan_1" {
		t.Fatalf("mark success id = %q, want %q", scanJobs.markSuccessID, "scan_1")
	}
	if scanJobs.markSuccessRawCount != 2 {
		t.Fatalf("mark success raw count = %d, want %d", scanJobs.markSuccessRawCount, 2)
	}
	if len(rawRepo.created) != 2 {
		t.Fatalf("raw listings persisted = %d, want %d", len(rawRepo.created), 2)
	}
	if rawRepo.created[0].NormalizedTitle == "" {
		t.Fatalf("expected normalized title to be set")
	}
	if len(groupedRepo.created) != 1 {
		t.Fatalf("grouped listings persisted = %d, want %d", len(groupedRepo.created), 1)
	}
	if snapshotRepo.created.ScanJobID != "scan_1" {
		t.Fatalf("snapshot scan job id = %q, want %q", snapshotRepo.created.ScanJobID, "scan_1")
	}
	if pricePointRepo.created.ScanJobID != "scan_1" {
		t.Fatalf("price point scan job id = %q, want %q", pricePointRepo.created.ScanJobID, "scan_1")
	}
	if len(alertRepo.created) != 1 {
		t.Fatalf("alert events persisted = %d, want %d", len(alertRepo.created), 1)
	}
	if alertRepo.created[0].EventType != domain.AlertEventTypeThresholdHit {
		t.Fatalf("alert event type = %q, want %q", alertRepo.created[0].EventType, domain.AlertEventTypeThresholdHit)
	}
	if result.ScanJob.Status != domain.ScanJobStatusSuccess {
		t.Fatalf("result scan status = %q, want %q", result.ScanJob.Status, domain.ScanJobStatusSuccess)
	}
}

func TestExecuteScraperFailureMarksScanFailed(t *testing.T) {
	clock := &stubClock{times: []time.Time{time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}}
	ids := &stubIDs{values: []string{"scan_1"}}
	scanJobs := &fakeScanJobRepo{}
	expectedErr := errors.New("scraper failed")

	service := NewService(
		stubScraper{err: expectedErr},
		nil,
		ids,
		clock,
		scanJobs,
		&fakeRawListingRepo{},
		&fakeGroupedListingRepo{},
		&fakeSnapshotRepo{},
		&fakePricePointRepo{},
		&fakeAlertEventRepo{},
		grouping.NewService(),
		snapshot.NewService(),
		history.NewService(),
		alert.NewService(),
	)

	_, err := service.Execute(context.Background(), domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if scanJobs.markFailedID != "scan_1" {
		t.Fatalf("mark failed id = %q, want %q", scanJobs.markFailedID, "scan_1")
	}
	if scanJobs.markFailedMessage != expectedErr.Error() {
		t.Fatalf("mark failed message = %q, want %q", scanJobs.markFailedMessage, expectedErr.Error())
	}
}

func TestExecuteNoDataStillCreatesSnapshotAndHistory(t *testing.T) {
	clock := &stubClock{
		times: []time.Time{
			time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 5, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 10, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 15, 0, time.UTC),
		},
	}
	ids := &stubIDs{values: []string{"scan_1", "snap_1", "pp_1"}}
	scanJobs := &fakeScanJobRepo{}
	snapshotRepo := &fakeSnapshotRepo{}
	pricePointRepo := &fakePricePointRepo{}
	alertRepo := &fakeAlertEventRepo{}

	service := NewService(
		stubScraper{listings: []domain.RawListing{}},
		nil,
		ids,
		clock,
		scanJobs,
		&fakeRawListingRepo{},
		&fakeGroupedListingRepo{},
		snapshotRepo,
		pricePointRepo,
		alertRepo,
		grouping.NewService(),
		snapshot.NewService(),
		history.NewService(),
		alert.NewService(),
	)

	result, err := service.Execute(context.Background(), domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Snapshot.Signal != domain.MarketSignalNoData {
		t.Fatalf("snapshot signal = %q, want %q", result.Snapshot.Signal, domain.MarketSignalNoData)
	}
	if result.Snapshot.MinPrice != nil || result.PricePoint.MinPrice != nil {
		t.Fatalf("expected nil min prices for no-data result")
	}
	if len(alertRepo.created) != 0 {
		t.Fatalf("alert events persisted = %d, want %d", len(alertRepo.created), 0)
	}
}

func TestExecuteDispatchesAlertNotificationsAfterPersist(t *testing.T) {
	clock := &stubClock{
		times: []time.Time{
			time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 5, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 10, 0, time.UTC),
			time.Date(2026, 4, 2, 10, 0, 15, 0, time.UTC),
		},
	}
	ids := &stubIDs{values: []string{"scan_1", "raw_1", "grp_1", "snap_1", "pp_1", "evt_1"}}
	alertRepo := &fakeAlertEventRepo{}
	dispatcher := &fakeAlertDispatcher{}
	service := NewService(
		stubScraper{
			listings: []domain.RawListing{
				{Title: "Bimoli Minyak Goreng 2L Promo", SellerName: "Seller A", Price: 22000, URL: "https://example.com/1", Source: "tokopedia"},
			},
		},
		dispatcher,
		ids,
		clock,
		&fakeScanJobRepo{},
		&fakeRawListingRepo{},
		&fakeGroupedListingRepo{},
		&fakeSnapshotRepo{},
		&fakePricePointRepo{},
		alertRepo,
		grouping.NewService(),
		snapshot.NewService(),
		history.NewService(),
		alert.NewService(),
	)

	threshold := int64(23000)
	_, err := service.Execute(context.Background(), domain.TrackedKeyword{
		ID:              "kw_1",
		Keyword:         "minyak goreng 2L",
		ThresholdPrice:  &threshold,
		TelegramEnabled: true,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if dispatcher.calls != 1 {
		t.Fatalf("dispatcher calls = %d, want 1", dispatcher.calls)
	}
	if len(dispatcher.alerts) != 1 || dispatcher.alerts[0].ID != "evt_1" {
		t.Fatalf("dispatched alerts = %#v", dispatcher.alerts)
	}
}

type stubScraper struct {
	listings []domain.RawListing
	err      error
}

func (s stubScraper) FetchListings(_ context.Context, _ domain.TrackedKeyword) ([]domain.RawListing, error) {
	if s.err != nil {
		return nil, s.err
	}

	listings := make([]domain.RawListing, len(s.listings))
	copy(listings, s.listings)
	return listings, nil
}

type stubIDs struct {
	values []string
	index  int
}

func (s *stubIDs) Next() string {
	if s.index >= len(s.values) {
		return fmt.Sprintf("generated_%d", s.index)
	}

	value := s.values[s.index]
	s.index++
	return value
}

type stubClock struct {
	times []time.Time
	index int
}

func (s *stubClock) Now() time.Time {
	if len(s.times) == 0 {
		return time.Time{}
	}

	if s.index >= len(s.times) {
		return s.times[len(s.times)-1]
	}

	value := s.times[s.index]
	s.index++
	return value
}

type fakeScanJobRepo struct {
	created             domain.ScanJob
	markSuccessID       string
	markSuccessRawCount int
	markSuccessGrpCount int
	markFailedID        string
	markFailedMessage   string
}

func (f *fakeScanJobRepo) Create(_ context.Context, scanJob domain.ScanJob) error {
	f.created = scanJob
	return nil
}

func (f *fakeScanJobRepo) MarkSuccess(_ context.Context, id string, rawCount, groupedCount int) error {
	f.markSuccessID = id
	f.markSuccessRawCount = rawCount
	f.markSuccessGrpCount = groupedCount
	return nil
}

func (f *fakeScanJobRepo) MarkFailed(_ context.Context, id string, errorMessage string) error {
	f.markFailedID = id
	f.markFailedMessage = errorMessage
	return nil
}

func (f *fakeScanJobRepo) GetLatestByKeywordID(_ context.Context, _ string) (*domain.ScanJob, error) {
	return nil, nil
}

func (f *fakeScanJobRepo) ListRunning(_ context.Context, _ int) ([]domain.ScanJob, error) {
	return nil, nil
}

type fakeRawListingRepo struct {
	created []domain.RawListing
}

func (f *fakeRawListingRepo) CreateBatch(_ context.Context, listings []domain.RawListing) error {
	f.created = append([]domain.RawListing{}, listings...)
	return nil
}

func (f *fakeRawListingRepo) ListByScanJobID(_ context.Context, _ string) ([]domain.RawListing, error) {
	return nil, nil
}

func (f *fakeRawListingRepo) PruneOlderThanScrapedAt(_ context.Context, _ time.Time) (int, error) {
	return 0, nil
}

type fakeGroupedListingRepo struct {
	created []domain.GroupedListing
}

func (f *fakeGroupedListingRepo) CreateBatch(_ context.Context, listings []domain.GroupedListing) error {
	f.created = append([]domain.GroupedListing{}, listings...)
	return nil
}

func (f *fakeGroupedListingRepo) ListByScanJobID(_ context.Context, _ string) ([]domain.GroupedListing, error) {
	return nil, nil
}

type fakeSnapshotRepo struct {
	created domain.MarketSnapshot
}

func (f *fakeSnapshotRepo) Create(_ context.Context, snapshot domain.MarketSnapshot) error {
	f.created = snapshot
	return nil
}

func (f *fakeSnapshotRepo) GetLatestByKeywordID(_ context.Context, _ string) (*domain.MarketSnapshot, error) {
	return nil, nil
}

type fakePricePointRepo struct {
	created       domain.PricePoint
	recentHistory []domain.PricePoint
}

func (f *fakePricePointRepo) Create(_ context.Context, point domain.PricePoint) error {
	f.created = point
	return nil
}

func (f *fakePricePointRepo) ListRecentByKeywordID(_ context.Context, _ string, _ int) ([]domain.PricePoint, error) {
	history := make([]domain.PricePoint, len(f.recentHistory))
	copy(history, f.recentHistory)
	return history, nil
}

type fakeAlertEventRepo struct {
	created      []domain.AlertEvent
	recentEvents []domain.AlertEvent
}

func (f *fakeAlertEventRepo) Create(_ context.Context, event domain.AlertEvent) error {
	f.created = append(f.created, event)
	return nil
}

func (f *fakeAlertEventRepo) MarkSentToTelegram(_ context.Context, _ string) error {
	return nil
}

func (f *fakeAlertEventRepo) ListRecentByKeywordID(_ context.Context, _ string, _ int) ([]domain.AlertEvent, error) {
	events := make([]domain.AlertEvent, len(f.recentEvents))
	copy(events, f.recentEvents)
	return events, nil
}

func (f *fakeAlertEventRepo) PruneOlderThanCreatedAt(_ context.Context, _ time.Time) (int, error) {
	return 0, nil
}

type fakeAlertDispatcher struct {
	calls  int
	alerts []domain.AlertEvent
}

func (f *fakeAlertDispatcher) DispatchActionable(_ context.Context, _ domain.TrackedKeyword, _ domain.MarketSnapshot, _ []domain.GroupedListing, alerts []domain.AlertEvent) {
	f.calls++
	f.alerts = append([]domain.AlertEvent(nil), alerts...)
}

func TestPrepareRawListingsSetsDefaults(t *testing.T) {
	clock := &stubClock{times: []time.Time{time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)}}
	ids := &stubIDs{values: []string{"raw_1"}}
	service := NewService(
		stubScraper{},
		nil,
		ids,
		clock,
		&fakeScanJobRepo{},
		&fakeRawListingRepo{},
		&fakeGroupedListingRepo{},
		&fakeSnapshotRepo{},
		&fakePricePointRepo{},
		&fakeAlertEventRepo{},
		grouping.NewService(),
		snapshot.NewService(),
		history.NewService(),
		alert.NewService(),
	)

	got := service.prepareRawListings("scan_1", []domain.RawListing{
		{Title: "Bimoli Minyak Goreng 2L Promo", Price: 22000},
	})

	want := []domain.RawListing{
		{
			ID:              "raw_1",
			ScanJobID:       "scan_1",
			Title:           "Bimoli Minyak Goreng 2L Promo",
			NormalizedTitle: "bimoli minyak goreng 2l",
			Price:           22000,
			ScrapedAt:       time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prepareRawListings() = %#v, want %#v", got, want)
	}
}
