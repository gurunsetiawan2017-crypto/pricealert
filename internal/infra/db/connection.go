package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pricealert/pricealert/internal/config"
)

// ConnectionConfig keeps DB bootstrap details in infrastructure code.
type ConnectionConfig struct {
	DriverName string
	DSN        string
}

func NewConnectionConfig(cfg config.DBConfig) (ConnectionConfig, error) {
	if cfg.Driver == "" {
		return ConnectionConfig{}, fmt.Errorf("db driver is required")
	}

	return ConnectionConfig{
		DriverName: cfg.Driver,
		DSN:        buildMariaDBDSN(cfg),
	}, nil
}

func Open(cfg ConnectionConfig) (*sql.DB, error) {
	return sql.Open(cfg.DriverName, cfg.DSN)
}

func OpenAndPing(ctx context.Context, cfg ConnectionConfig) (*sql.DB, error) {
	db, err := Open(cfg)
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return db, nil
}

func buildMariaDBDSN(cfg config.DBConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
		cfg.Params,
	)
}
