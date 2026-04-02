package app

import (
	"context"
	"database/sql"
	"time"

	"github.com/pricealert/pricealert/internal/config"
	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/infra/idgen"
	infNotifier "github.com/pricealert/pricealert/internal/infra/notifier"
	infScraper "github.com/pricealert/pricealert/internal/infra/scraper"
	"github.com/pricealert/pricealert/internal/repository"
	rtscheduler "github.com/pricealert/pricealert/internal/runtime/scheduler"
	rtstate "github.com/pricealert/pricealert/internal/runtime/state"
	rtworker "github.com/pricealert/pricealert/internal/runtime/worker"
	"github.com/pricealert/pricealert/internal/service/alert"
	"github.com/pricealert/pricealert/internal/service/grouping"
	"github.com/pricealert/pricealert/internal/service/history"
	notifyservice "github.com/pricealert/pricealert/internal/service/notifier"
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
	scheduler      *rtscheduler.Scheduler
	worker         *rtworker.Worker
	state          *rtstate.Store
	startup        StartupReconciliationResult
	pruning        RawListingPruneResult
	alertPruning   AlertEventPruneResult
}

func (r *Runtime) RunOnce(ctx context.Context) (rtscheduler.RunResult, error) {
	return r.scheduler.RunOnce(ctx)
}

type RuntimeStatus struct {
	AcceptingNewWork       bool
	MaxConcurrent          int
	RunningCount           int
	TrackedKeywords        int
	ReconciledRunningJobs  int
	LastReconciledAt       *time.Time
	PrunedRawListings      int
	LastPrunedAt           *time.Time
	PrunedAlertEvents      int
	LastAlertPrunedAt      *time.Time
}

func (r *Runtime) Status() RuntimeStatus {
	workerStatus := r.worker.Status()
	stateSummary := r.state.Summary()

	return RuntimeStatus{
		AcceptingNewWork:       workerStatus.AcceptingNewWork,
		MaxConcurrent:          workerStatus.MaxConcurrent,
		RunningCount:           stateSummary.RunningCount,
		TrackedKeywords:        stateSummary.KeywordsTracked,
		ReconciledRunningJobs:  r.startup.ReconciledCount,
		LastReconciledAt:       r.startup.ReconciledAt,
		PrunedRawListings:      r.pruning.PrunedCount,
		LastPrunedAt:           r.pruning.PrunedAt,
		PrunedAlertEvents:      r.alertPruning.PrunedCount,
		LastAlertPrunedAt:      r.alertPruning.PrunedAt,
	}
}

func (r *Runtime) RuntimeStatus() RuntimeStatus {
	return r.Status()
}

func (r *Runtime) Close(ctx context.Context) error {
	r.worker.StopAcceptingNewWork()
	return r.worker.Wait(ctx)
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

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now().UTC()
}

const abandonedScanJobReason = "startup reconciliation: previous app run ended before scan completed"

type StartupReconciliationResult struct {
	ReconciledCount int
	ReconciledAt    *time.Time
}

type RawListingPruneResult struct {
	PrunedCount int
	PrunedAt    *time.Time
}

type AlertEventPruneResult struct {
	PrunedCount int
	PrunedAt    *time.Time
}

func reconcileAbandonedRunningScanJobs(ctx context.Context, scanJobs repository.ScanJobRepository, clock Clock) (StartupReconciliationResult, error) {
	running, err := scanJobs.ListRunning(ctx, 1024)
	if err != nil {
		return StartupReconciliationResult{}, err
	}
	if len(running) == 0 {
		now := clock.Now()
		return StartupReconciliationResult{ReconciledAt: &now}, nil
	}

	for _, scanJob := range running {
		if err := scanJobs.MarkFailed(ctx, scanJob.ID, abandonedScanJobReason); err != nil {
			return StartupReconciliationResult{}, err
		}
	}

	now := clock.Now()
	return StartupReconciliationResult{
		ReconciledCount: len(running),
		ReconciledAt:    &now,
	}, nil
}

func pruneRawListings(ctx context.Context, rawListings repository.RawListingRepository, retentionHours int, clock Clock) (RawListingPruneResult, error) {
	if retentionHours <= 0 {
		return RawListingPruneResult{}, nil
	}

	now := clock.Now()
	cutoff := now.Add(-time.Duration(retentionHours) * time.Hour)
	pruned, err := rawListings.PruneOlderThanScrapedAt(ctx, cutoff)
	if err != nil {
		return RawListingPruneResult{}, err
	}

	return RawListingPruneResult{
		PrunedCount: pruned,
		PrunedAt:    &now,
	}, nil
}

func pruneAlertEvents(ctx context.Context, alertEvents repository.AlertEventRepository, retentionHours int, clock Clock) (AlertEventPruneResult, error) {
	if retentionHours <= 0 {
		return AlertEventPruneResult{}, nil
	}

	now := clock.Now()
	cutoff := now.Add(-time.Duration(retentionHours) * time.Hour)
	pruned, err := alertEvents.PruneOlderThanCreatedAt(ctx, cutoff)
	if err != nil {
		return AlertEventPruneResult{}, err
	}

	return AlertEventPruneResult{
		PrunedCount: pruned,
		PrunedAt:    &now,
	}, nil
}

func newRuntime(cfg config.Config, repos appRepositories) *Runtime {
	return newRuntimeWithClock(cfg, repos, systemClock{})
}

func newRuntimeWithClock(cfg config.Config, repos appRepositories, clock Clock) *Runtime {
	idGenerator := idgen.NewULIDGenerator()
	telegramSender := newTelegramSender(cfg)
	notifierService := notifyservice.NewService(telegramSender, idGenerator, clock, repos.alertEvents)
	scanService := scan.NewService(
		infScraper.NewTokopedia(cfg.Scraper),
		notifierService,
		idGenerator,
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
	worker := rtworker.New(stateStore, scanExecutorAdapter{scan: scanService}, clock, cfg.Runtime.MaxConcurrentScans)
	scheduler := rtscheduler.New(trackedKeywordSourceAdapter{repo: repos.trackedKeywords}, stateStore, worker, clock)

	return &Runtime{scheduler: scheduler, worker: worker, state: stateStore}
}

func newTelegramSender(cfg config.Config) notifyservice.Sender {
	if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
		return infNotifier.NewNoop()
	}

	return infNotifier.NewTelegram(cfg.Telegram)
}
