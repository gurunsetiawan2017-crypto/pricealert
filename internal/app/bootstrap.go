package app

import (
	"context"
	"database/sql"

	"github.com/pricealert/pricealert/internal/config"
	infraDB "github.com/pricealert/pricealert/internal/infra/db"
	rtscheduler "github.com/pricealert/pricealert/internal/runtime/scheduler"
	"github.com/pricealert/pricealert/internal/service/query"
)

// App wires the current local runtime foundation and shared infrastructure.
type App struct {
	cfg        config.Config
	db         *sql.DB
	dbConfig   infraDB.ConnectionConfig
	migrations []infraDB.Migration

	repos   appRepositories
	queries *query.Service
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
		queries:    newQueryService(repos),
		runtime:    newRuntime(cfg, repos),
	}, nil
}

func (a *App) Run() error {
	defer a.db.Close()

	_, err := newTUIProgram(a.queries).Run()
	return err
}

func (a *App) RunRuntimeOnce(ctx context.Context) (rtscheduler.RunResult, error) {
	return a.runtime.RunOnce(ctx)
}
