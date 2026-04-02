package app

import (
	"context"
	"database/sql"

	"github.com/pricealert/pricealert/internal/config"
	infraDB "github.com/pricealert/pricealert/internal/infra/db"
	"github.com/pricealert/pricealert/internal/repository"
)

// App is a thin bootstrap shell for milestone A.
type App struct {
	cfg        config.Config
	db         *sql.DB
	dbConfig   infraDB.ConnectionConfig
	migrations []infraDB.Migration

	trackedKeywords repository.TrackedKeywordRepository
	scanJobs        repository.ScanJobRepository
	rawListings     repository.RawListingRepository
	groupedListings repository.GroupedListingRepository
	snapshots       repository.MarketSnapshotRepository
	alertEvents     repository.AlertEventRepository
}

func New(cfg config.Config) (*App, error) {
	dbConfig, err := infraDB.NewConnectionConfig(cfg.DB)
	if err != nil {
		return nil, err
	}

	migrations, err := infraDB.DiscoverMigrations(cfg.Paths.MigrationsDir)
	if err != nil {
		return nil, err
	}

	db, err := infraDB.OpenAndPing(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}

	return &App{
		cfg:             cfg,
		db:              db,
		dbConfig:        dbConfig,
		migrations:      migrations,
		trackedKeywords: repository.NewMariaDBTrackedKeywordRepository(db),
		scanJobs:        repository.NewMariaDBScanJobRepository(db),
		rawListings:     repository.NewMariaDBRawListingRepository(db),
		groupedListings: repository.NewMariaDBGroupedListingRepository(db),
		snapshots:       repository.NewMariaDBMarketSnapshotRepository(db),
		alertEvents:     repository.NewMariaDBAlertEventRepository(db),
	}, nil
}

func (a *App) Run() error {
	// Runtime/TUI/worker wiring is intentionally deferred to later milestones.
	_ = a.cfg
	defer a.db.Close()
	_ = a.dbConfig
	_ = a.migrations
	_ = a.trackedKeywords
	_ = a.scanJobs
	_ = a.rawListings
	_ = a.groupedListings
	_ = a.snapshots
	_ = a.alertEvents
	return nil
}
