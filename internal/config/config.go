package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	defaultAppName       = "pricealert"
	defaultRuntimeTZ     = "UTC"
	defaultLogLevel      = "info"
	defaultMinInterval   = 5
	defaultMaxConcurrent = 1
	defaultDBDriver      = "mysql"
	defaultMigrationsDir = "migrations"
)

// Config holds runtime configuration for the single-process v1 application.
type Config struct {
	AppName string
	Runtime RuntimeConfig
	DB      DBConfig
	Paths   PathsConfig
}

type RuntimeConfig struct {
	Timezone            string
	LogLevel            string
	MinScanIntervalMins int
	MaxConcurrentScans  int
}

type DBConfig struct {
	Driver   string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	Params   string
}

type PathsConfig struct {
	MigrationsDir string
}

// Load reads configuration from environment variables and applies safe defaults
// for milestone-A scaffolding.
func Load() (Config, error) {
	minScanInterval, err := getEnvInt("PRICEALERT_MIN_SCAN_INTERVAL_MINS", defaultMinInterval)
	if err != nil {
		return Config{}, err
	}

	maxConcurrentScans, err := getEnvInt("PRICEALERT_MAX_CONCURRENT_SCANS", defaultMaxConcurrent)
	if err != nil {
		return Config{}, err
	}

	dbPort, err := getEnvInt("PRICEALERT_DB_PORT", 3306)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppName: getEnv("PRICEALERT_APP_NAME", defaultAppName),
		Runtime: RuntimeConfig{
			Timezone:            getEnv("PRICEALERT_RUNTIME_TIMEZONE", defaultRuntimeTZ),
			LogLevel:            getEnv("PRICEALERT_LOG_LEVEL", defaultLogLevel),
			MinScanIntervalMins: minScanInterval,
			MaxConcurrentScans:  maxConcurrentScans,
		},
		DB: DBConfig{
			Driver:   getEnv("PRICEALERT_DB_DRIVER", defaultDBDriver),
			Host:     getEnv("PRICEALERT_DB_HOST", "127.0.0.1"),
			Port:     dbPort,
			User:     getEnv("PRICEALERT_DB_USER", "root"),
			Password: getEnv("PRICEALERT_DB_PASSWORD", ""),
			Name:     getEnv("PRICEALERT_DB_NAME", "pricealert"),
			Params:   getEnv("PRICEALERT_DB_PARAMS", "parseTime=true&charset=utf8mb4&loc=UTC"),
		},
		Paths: PathsConfig{
			MigrationsDir: getEnv("PRICEALERT_MIGRATIONS_DIR", defaultMigrationsDir),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.Runtime.MinScanIntervalMins <= 0 {
		return fmt.Errorf("runtime min scan interval must be > 0")
	}

	if c.Runtime.MaxConcurrentScans <= 0 {
		return fmt.Errorf("runtime max concurrent scans must be > 0")
	}

	if c.DB.Port < 1 || c.DB.Port > 65535 {
		return fmt.Errorf("db port must be between 1 and 65535")
	}

	if c.DB.Driver == "" || c.DB.Host == "" || c.DB.User == "" || c.DB.Name == "" {
		return fmt.Errorf("db driver, host, user, and name are required")
	}

	if c.Paths.MigrationsDir == "" {
		return fmt.Errorf("migrations dir is required")
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %q", key, value)
	}

	return parsed, nil
}
