package app

import (
	"context"
	"database/sql"
	"errors"
	"sync"
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
	scheduler       *rtscheduler.Scheduler
	worker          *rtworker.Worker
	state           *rtstate.Store
	trackedKeywords repository.TrackedKeywordRepository
	startup         StartupReconciliationResult
	pruning         RawListingPruneResult
	alertPruning    AlertEventPruneResult
}

func (r *Runtime) RunOnce(ctx context.Context) (rtscheduler.RunResult, error) {
	return r.scheduler.RunOnce(ctx)
}

func (r *Runtime) ScanKeywordNow(ctx context.Context, keywordID string) error {
	keyword, err := r.trackedKeywords.GetByID(ctx, keywordID)
	if err != nil {
		return err
	}
	if keyword == nil {
		return sql.ErrNoRows
	}
	if keyword.Status == domain.TrackedKeywordStatusArchived {
		return errKeywordArchived
	}

	return r.worker.ExecuteNow(ctx, *keyword)
}

type RuntimeStatus struct {
	AcceptingNewWork      bool
	MaxConcurrent         int
	RunningCount          int
	TrackedKeywords       int
	FailedKeywords        int
	LatestFailureMessage  *string
	LastFailureAt         *time.Time
	ReconciledRunningJobs int
	LastReconciledAt      *time.Time
	PrunedRawListings     int
	LastPrunedAt          *time.Time
	PrunedAlertEvents     int
	LastAlertPrunedAt     *time.Time
}

type RuntimeKeywordStatus struct {
	Running          bool
	LastSuccessAt    *time.Time
	LastErrorMessage *string
	LastErrorAt      *time.Time
}

func (r *Runtime) Status() RuntimeStatus {
	workerStatus := r.worker.Status()
	stateSummary := r.state.Summary()

	return RuntimeStatus{
		AcceptingNewWork:      workerStatus.AcceptingNewWork,
		MaxConcurrent:         workerStatus.MaxConcurrent,
		RunningCount:          stateSummary.RunningCount,
		TrackedKeywords:       stateSummary.KeywordsTracked,
		FailedKeywords:        stateSummary.FailedKeywords,
		LatestFailureMessage:  stateSummary.LatestFailure,
		LastFailureAt:         stateSummary.LatestFailureAt,
		ReconciledRunningJobs: r.startup.ReconciledCount,
		LastReconciledAt:      r.startup.ReconciledAt,
		PrunedRawListings:     r.pruning.PrunedCount,
		LastPrunedAt:          r.pruning.PrunedAt,
		PrunedAlertEvents:     r.alertPruning.PrunedCount,
		LastAlertPrunedAt:     r.alertPruning.PrunedAt,
	}
}

func (r *Runtime) RuntimeStatus() RuntimeStatus {
	return r.Status()
}

func (r *Runtime) KeywordRuntimeStatus(keywordID string) RuntimeKeywordStatus {
	state := r.state.Snapshot(keywordID)
	return RuntimeKeywordStatus{
		Running:          state.Running,
		LastSuccessAt:    copyTime(state.LastSuccessAt),
		LastErrorMessage: copyString(state.LastError),
		LastErrorAt:      copyTime(firstNonNilTimeValue(state.LastFinishedAt, state.LastAttemptAt)),
	}
}

func (r *Runtime) Close(ctx context.Context) error {
	r.worker.StopAcceptingNewWork()
	return r.worker.Wait(ctx)
}

type runtimeStepRunner interface {
	RunOnce(context.Context) (rtscheduler.RunResult, error)
}

type ticker interface {
	C() <-chan time.Time
	Stop()
}

type timeTicker struct {
	ticker *time.Ticker
}

func newTimeTicker(interval time.Duration) ticker {
	return &timeTicker{ticker: time.NewTicker(interval)}
}

func (t *timeTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *timeTicker) Stop() {
	t.ticker.Stop()
}

type runtimeLoop struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func startRuntimeLoop(parent context.Context, runner runtimeStepRunner, interval time.Duration, newTicker func(time.Duration) ticker) *runtimeLoop {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if newTicker == nil {
		newTicker = newTimeTicker
	}

	ctx, cancel := context.WithCancel(parent)
	loop := &runtimeLoop{
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		defer close(loop.done)

		ticker := newTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C():
				_, _ = runner.RunOnce(context.Background())
			}
		}
	}()

	return loop
}

func (l *runtimeLoop) Stop(ctx context.Context) error {
	if l == nil {
		return nil
	}

	l.once.Do(func() {
		l.cancel()
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.done:
		return nil
	}
}

func autonomousRuntimeLoopInterval(cfg config.Config) time.Duration {
	base := time.Duration(cfg.Runtime.MinScanIntervalMins) * time.Minute
	interval := base / 6
	if interval < 5*time.Second {
		return 5 * time.Second
	}
	if interval > 30*time.Second {
		return 30 * time.Second
	}
	return interval
}

type trackedKeywordSourceAdapter struct {
	repo repository.TrackedKeywordRepository
}

func (a trackedKeywordSourceAdapter) ListKeywords(ctx context.Context) ([]domain.TrackedKeyword, error) {
	return a.repo.ListVisible(ctx)
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

var errKeywordArchived = errors.New("archived keyword cannot be scanned")

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

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func firstNonNilTimeValue(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func reconcileAbandonedRunningScanJobs(ctx context.Context, scanJobs repository.ScanJobRepository, clock Clock) (StartupReconciliationResult, error) {
	now := clock.Now()
	reconciledCount := 0

	for {
		running, err := scanJobs.ListRunning(ctx, 1024)
		if err != nil {
			return StartupReconciliationResult{}, err
		}
		if len(running) == 0 {
			return StartupReconciliationResult{
				ReconciledCount: reconciledCount,
				ReconciledAt:    &now,
			}, nil
		}

		for _, scanJob := range running {
			if err := scanJobs.MarkFailed(ctx, scanJob.ID, abandonedScanJobReason); err != nil {
				return StartupReconciliationResult{}, err
			}
		}
		reconciledCount += len(running)
	}
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

	return &Runtime{
		scheduler:       scheduler,
		worker:          worker,
		state:           stateStore,
		trackedKeywords: repos.trackedKeywords,
	}
}

func newTelegramSender(cfg config.Config) notifyservice.Sender {
	if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
		return infNotifier.NewNoop()
	}

	return infNotifier.NewTelegram(cfg.Telegram)
}
