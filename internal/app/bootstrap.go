package app

import (
	"context"
	"database/sql"

	"github.com/pricealert/pricealert/internal/config"
	infraDB "github.com/pricealert/pricealert/internal/infra/db"
	rtscheduler "github.com/pricealert/pricealert/internal/runtime/scheduler"
)

// App wires the current local runtime foundation and shared infrastructure.
type App struct {
	cfg        config.Config
	db         *sql.DB
	dbConfig   infraDB.ConnectionConfig
	migrations []infraDB.Migration

	repos   appRepositories
	runtime *Runtime
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

	repos := newAppRepositories(db)

	return &App{
		cfg:        cfg,
		db:         db,
		dbConfig:   dbConfig,
		migrations: migrations,
		repos:      repos,
		runtime:    newRuntime(repos),
	}, nil
}

func (a *App) Run() error {
	// Runtime execution remains explicit and bounded via RunRuntimeOnce.
	_ = a.cfg
	defer a.db.Close()
	_ = a.dbConfig
	_ = a.migrations
	_ = a.repos
	_ = a.runtime
	return nil
}

func (a *App) RunRuntimeOnce(ctx context.Context) (rtscheduler.RunResult, error) {
	return a.runtime.RunOnce(ctx)
}
