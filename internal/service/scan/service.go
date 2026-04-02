package scan

import (
	"context"
	"fmt"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/repository"
	"github.com/pricealert/pricealert/internal/service/alert"
	"github.com/pricealert/pricealert/internal/service/grouping"
	"github.com/pricealert/pricealert/internal/service/history"
	"github.com/pricealert/pricealert/internal/service/snapshot"
)

type IDGenerator interface {
	Next() string
}

type Clock interface {
	Now() time.Time
}

type Scraper interface {
	FetchListings(context.Context, domain.TrackedKeyword) ([]domain.RawListing, error)
}

type AlertDispatcher interface {
	DispatchActionable(context.Context, domain.TrackedKeyword, domain.MarketSnapshot, []domain.GroupedListing, []domain.AlertEvent)
}

type Service struct {
	scraper     Scraper
	notifier    AlertDispatcher
	idGenerator IDGenerator
	clock       Clock
	scanJobs    repository.ScanJobRepository
	rawListings repository.RawListingRepository
	grouped     repository.GroupedListingRepository
	snapshots   repository.MarketSnapshotRepository
	pricePoints repository.PricePointRepository
	alertEvents repository.AlertEventRepository
	grouping    *grouping.Service
	snapshot    *snapshot.Service
	history     *history.Service
	alert       *alert.Service
}

type Result struct {
	ScanJob     domain.ScanJob
	RawListings []domain.RawListing
	Grouped     []domain.GroupedListing
	Snapshot    domain.MarketSnapshot
	PricePoint  domain.PricePoint
	AlertEvents []domain.AlertEvent
}

func NewService(
	scraper Scraper,
	notifier AlertDispatcher,
	idGenerator IDGenerator,
	clock Clock,
	scanJobs repository.ScanJobRepository,
	rawListings repository.RawListingRepository,
	grouped repository.GroupedListingRepository,
	snapshots repository.MarketSnapshotRepository,
	pricePoints repository.PricePointRepository,
	alertEvents repository.AlertEventRepository,
	groupingService *grouping.Service,
	snapshotService *snapshot.Service,
	historyService *history.Service,
	alertService *alert.Service,
) *Service {
	return &Service{
		scraper:     scraper,
		notifier:    notifier,
		idGenerator: idGenerator,
		clock:       clock,
		scanJobs:    scanJobs,
		rawListings: rawListings,
		grouped:     grouped,
		snapshots:   snapshots,
		pricePoints: pricePoints,
		alertEvents: alertEvents,
		grouping:    groupingService,
		snapshot:    snapshotService,
		history:     historyService,
		alert:       alertService,
	}
}

func (s *Service) Execute(ctx context.Context, keyword domain.TrackedKeyword) (*Result, error) {
	startedAt := s.clock.Now()
	scanJob := domain.ScanJob{
		ID:               s.idGenerator.Next(),
		TrackedKeywordID: keyword.ID,
		StartedAt:        startedAt,
		Status:           domain.ScanJobStatusRunning,
	}

	if err := s.scanJobs.Create(ctx, scanJob); err != nil {
		return nil, fmt.Errorf("create scan job: %w", err)
	}

	rawListings, err := s.scraper.FetchListings(ctx, keyword)
	if err != nil {
		_ = s.scanJobs.MarkFailed(ctx, scanJob.ID, err.Error())
		return nil, fmt.Errorf("fetch listings: %w", err)
	}

	rawListings = s.prepareRawListings(scanJob.ID, rawListings)
	if err := s.rawListings.CreateBatch(ctx, rawListings); err != nil {
		_ = s.scanJobs.MarkFailed(ctx, scanJob.ID, err.Error())
		return nil, fmt.Errorf("persist raw listings: %w", err)
	}

	groupedListings := s.grouping.Group(scanJob.ID, rawListings)
	groupedListings = s.assignGroupedListingIDs(groupedListings)
	if err := s.grouped.CreateBatch(ctx, groupedListings); err != nil {
		_ = s.scanJobs.MarkFailed(ctx, scanJob.ID, err.Error())
		return nil, fmt.Errorf("persist grouped listings: %w", err)
	}

	snapshotAt := s.clock.Now()
	marketSnapshot := s.snapshot.Build(keyword.ID, scanJob.ID, len(rawListings), groupedListings, snapshotAt)
	marketSnapshot.ID = s.idGenerator.Next()
	if err := s.snapshots.Create(ctx, marketSnapshot); err != nil {
		_ = s.scanJobs.MarkFailed(ctx, scanJob.ID, err.Error())
		return nil, fmt.Errorf("persist market snapshot: %w", err)
	}

	pricePoint := s.history.BuildFromSnapshot(marketSnapshot)
	pricePoint.ID = s.idGenerator.Next()
	if err := s.pricePoints.Create(ctx, pricePoint); err != nil {
		_ = s.scanJobs.MarkFailed(ctx, scanJob.ID, err.Error())
		return nil, fmt.Errorf("persist price point: %w", err)
	}

	recentHistory, err := s.pricePoints.ListRecentByKeywordID(ctx, keyword.ID, 32)
	if err != nil {
		_ = s.scanJobs.MarkFailed(ctx, scanJob.ID, err.Error())
		return nil, fmt.Errorf("load recent history: %w", err)
	}

	recentEvents, err := s.alertEvents.ListRecentByKeywordID(ctx, keyword.ID, 32)
	if err != nil {
		_ = s.scanJobs.MarkFailed(ctx, scanJob.ID, err.Error())
		return nil, fmt.Errorf("load recent alert events: %w", err)
	}

	alerts := s.alert.Evaluate(keyword, marketSnapshot, recentHistory, recentEvents)
	for index := range alerts {
		alerts[index].ID = s.idGenerator.Next()
		if err := s.alertEvents.Create(ctx, alerts[index]); err != nil {
			_ = s.scanJobs.MarkFailed(ctx, scanJob.ID, err.Error())
			return nil, fmt.Errorf("persist alert event: %w", err)
		}
	}

	if s.notifier != nil {
		s.notifier.DispatchActionable(ctx, keyword, marketSnapshot, groupedListings, alerts)
	}

	if err := s.scanJobs.MarkSuccess(ctx, scanJob.ID, len(rawListings), len(groupedListings)); err != nil {
		return nil, fmt.Errorf("mark scan success: %w", err)
	}

	scanJob.Status = domain.ScanJobStatusSuccess
	finishedAt := s.clock.Now()
	scanJob.FinishedAt = &finishedAt
	scanJob.RawCount = len(rawListings)
	scanJob.GroupedCount = len(groupedListings)

	return &Result{
		ScanJob:     scanJob,
		RawListings: rawListings,
		Grouped:     groupedListings,
		Snapshot:    marketSnapshot,
		PricePoint:  pricePoint,
		AlertEvents: alerts,
	}, nil
}

func (s *Service) prepareRawListings(scanJobID string, listings []domain.RawListing) []domain.RawListing {
	prepared := make([]domain.RawListing, 0, len(listings))
	scrapedAt := s.clock.Now()

	for _, listing := range listings {
		if listing.ID == "" {
			listing.ID = s.idGenerator.Next()
		}
		listing.ScanJobID = scanJobID
		listing.NormalizedTitle = grouping.NormalizeTitle(listing.Title)
		if listing.ScrapedAt.IsZero() {
			listing.ScrapedAt = scrapedAt
		}

		prepared = append(prepared, listing)
	}

	return prepared
}

func (s *Service) assignGroupedListingIDs(listings []domain.GroupedListing) []domain.GroupedListing {
	for index := range listings {
		if listings[index].ID == "" {
			listings[index].ID = s.idGenerator.Next()
		}
	}

	return listings
}
