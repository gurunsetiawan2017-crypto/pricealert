package app

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/infra/idgen"
	"github.com/pricealert/pricealert/internal/repository"
	rtscheduler "github.com/pricealert/pricealert/internal/runtime/scheduler"
	rtstate "github.com/pricealert/pricealert/internal/runtime/state"
	rtworker "github.com/pricealert/pricealert/internal/runtime/worker"
	"github.com/pricealert/pricealert/internal/service/alert"
	"github.com/pricealert/pricealert/internal/service/grouping"
	"github.com/pricealert/pricealert/internal/service/history"
	"github.com/pricealert/pricealert/internal/service/scan"
	"github.com/pricealert/pricealert/internal/service/snapshot"
)

type appRepositories struct {
	trackedKeywords repository.TrackedKeywordRepository
	scanJobs        repository.ScanJobRepository
	rawListings     repository.RawListingRepository
	groupedListings repository.GroupedListingRepository
	snapshots       repository.MarketSnapshotRepository
	pricePoints     repository.PricePointRepository
	alertEvents     repository.AlertEventRepository
}

func newAppRepositories(db *sql.DB) appRepositories {
	return appRepositories{
		trackedKeywords: repository.NewMariaDBTrackedKeywordRepository(db),
		scanJobs:        repository.NewMariaDBScanJobRepository(db),
		rawListings:     repository.NewMariaDBRawListingRepository(db),
		groupedListings: repository.NewMariaDBGroupedListingRepository(db),
		snapshots:       repository.NewMariaDBMarketSnapshotRepository(db),
		pricePoints:     repository.NewMariaDBPricePointRepository(db),
		alertEvents:     repository.NewMariaDBAlertEventRepository(db),
	}
}

type Runtime struct {
	scheduler *rtscheduler.Scheduler
}

func (r *Runtime) RunOnce(ctx context.Context) (rtscheduler.RunResult, error) {
	return r.scheduler.RunOnce(ctx)
}

type trackedKeywordSourceAdapter struct {
	repo repository.TrackedKeywordRepository
}

func (a trackedKeywordSourceAdapter) ListKeywords(ctx context.Context) ([]domain.TrackedKeyword, error) {
	return a.repo.ListActive(ctx)
}

type scanRunner interface {
	Execute(context.Context, domain.TrackedKeyword) (*scan.Result, error)
}

type scanExecutorAdapter struct {
	scan scanRunner
}

func (a scanExecutorAdapter) Execute(ctx context.Context, keyword domain.TrackedKeyword) error {
	_, err := a.scan.Execute(ctx, keyword)
	return err
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now().UTC()
}

type unsupportedScraper struct{}

func (unsupportedScraper) FetchListings(context.Context, domain.TrackedKeyword) ([]domain.RawListing, error) {
	return nil, errors.New("tokopedia scraper not implemented yet")
}

func newRuntime(repos appRepositories) *Runtime {
	clock := systemClock{}
	scanService := scan.NewService(
		unsupportedScraper{},
		idgen.NewULIDGenerator(),
		clock,
		repos.scanJobs,
		repos.rawListings,
		repos.groupedListings,
		repos.snapshots,
		repos.pricePoints,
		repos.alertEvents,
		grouping.NewService(),
		snapshot.NewService(),
		history.NewService(),
		alert.NewService(),
	)

	stateStore := rtstate.NewStore()
	worker := rtworker.New(stateStore, scanExecutorAdapter{scan: scanService}, clock)
	scheduler := rtscheduler.New(trackedKeywordSourceAdapter{repo: repos.trackedKeywords}, stateStore, worker, clock)

	return &Runtime{scheduler: scheduler}
}
