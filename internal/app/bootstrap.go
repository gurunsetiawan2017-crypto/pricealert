package app

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"time"

	"github.com/pricealert/pricealert/internal/config"
	infraDB "github.com/pricealert/pricealert/internal/infra/db"
	rtscheduler "github.com/pricealert/pricealert/internal/runtime/scheduler"
	kwservice "github.com/pricealert/pricealert/internal/service/keyword"
	"github.com/pricealert/pricealert/internal/service/query"
)

// App wires the current local runtime foundation and shared infrastructure.
type App struct {
	cfg        config.Config
	db         *sql.DB
	dbConfig   infraDB.ConnectionConfig
	migrations []infraDB.Migration

	repos    appRepositories
	queries  *query.Service
	keywords *kwservice.Service
	runtime  *Runtime
}

func New(cfg config.Config) (*App, error) {
	dbConfig, err := infraDB.NewConnectionConfig(cfg.DB)
	if err != nil {
		return nil, err
	}

	migrations, err := discoverMigrationsIfPresent(cfg.Paths.MigrationsDir)
	if err != nil {
		return nil, err
	}

	db, err := infraDB.OpenAndPing(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}

	repos := newAppRepositories(db)
	clock := systemClock{}
	startup, err := reconcileAbandonedRunningScanJobs(context.Background(), repos.scanJobs, clock)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	pruning, err := pruneRawListings(context.Background(), repos.rawListings, cfg.Retention.RawListingsHours, clock)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	alertPruning, err := pruneAlertEvents(context.Background(), repos.alertEvents, cfg.Retention.AlertEventsHours, clock)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	runtime := newRuntimeWithClock(cfg, repos, clock)
	runtime.startup = startup
	runtime.pruning = pruning
	runtime.alertPruning = alertPruning

	return &App{
		cfg:        cfg,
		db:         db,
		dbConfig:   dbConfig,
		migrations: migrations,
		repos:      repos,
		queries:    newQueryService(repos, newRuntimeStatusAdapter(runtime)),
		keywords:   newKeywordService(repos, cfg.Runtime.MinScanIntervalMins),
		runtime:    runtime,
	}, nil
}

func (a *App) Run() error {
	runtimeLoop := startRuntimeLoop(context.Background(), a.runtime, autonomousRuntimeLoopInterval(a.cfg), nil)

	defer func() {
		loopCtx, loopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer loopCancel()
		_ = runtimeLoop.Stop(loopCtx)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.runtime.Close(shutdownCtx)
		_ = a.db.Close()
	}()

	_, err := newTUIProgram(a.queries, newRuntimeTrigger(a), newBrowserOpener(), newKeywordActions(a.keywords)).Run()
	return err
}

func (a *App) RunRuntimeOnce(ctx context.Context) (rtscheduler.RunResult, error) {
	return a.runtime.RunOnce(ctx)
}

func (a *App) ScanKeywordNow(ctx context.Context, keywordID string) error {
	return a.runtime.ScanKeywordNow(ctx, keywordID)
}

func (a *App) RuntimeStatus() RuntimeStatus {
	return a.runtime.Status()
}

func discoverMigrationsIfPresent(dir string) ([]infraDB.Migration, error) {
	migrations, err := infraDB.DiscoverMigrations(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return migrations, nil
}
