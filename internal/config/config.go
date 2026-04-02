package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	defaultAppName                 = "pricealert"
	defaultRuntimeTZ               = "UTC"
	defaultLogLevel                = "info"
	defaultMinInterval             = 5
	defaultMaxConcurrent           = 1
	defaultDBDriver                = "mysql"
	defaultMigrationsDir           = "migrations"
	defaultScraperRows             = 10
	defaultScraperTimeoutSeconds   = 20
	defaultScraperRetryAttempts    = 3
	defaultScraperRetryBackoffMS   = 500
	defaultTokopediaSearchEndpoint = "https://gql.tokopedia.com/graphql/SearchProductV5Query"
	defaultTelegramAPIBaseURL      = "https://api.telegram.org"
	defaultTelegramTimeoutSeconds  = 10
	defaultRawListingRetentionHrs  = 24 * 14
	defaultAlertEventRetentionHrs  = 24 * 30
)

// Config holds runtime configuration for the single-process v1 application.
type Config struct {
	AppName   string
	Runtime   RuntimeConfig
	DB        DBConfig
	Paths     PathsConfig
	Scraper   ScraperConfig
	Telegram  TelegramConfig
	Retention RetentionConfig
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

type ScraperConfig struct {
	TokopediaSearchEndpoint string
	TimeoutSeconds          int
	RowsPerScan             int
	RetryAttempts           int
	RetryBackoffMillis      int
}

type TelegramConfig struct {
	BotToken       string
	ChatID         string
	APIBaseURL     string
	TimeoutSeconds int
}

type RetentionConfig struct {
	RawListingsHours int
	AlertEventsHours int
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

	scraperTimeoutSeconds, err := getEnvInt("PRICEALERT_SCRAPER_TIMEOUT_SECONDS", defaultScraperTimeoutSeconds)
	if err != nil {
		return Config{}, err
	}

	scraperRows, err := getEnvInt("PRICEALERT_SCRAPER_ROWS_PER_SCAN", defaultScraperRows)
	if err != nil {
		return Config{}, err
	}

	scraperRetryAttempts, err := getEnvInt("PRICEALERT_SCRAPER_RETRY_ATTEMPTS", defaultScraperRetryAttempts)
	if err != nil {
		return Config{}, err
	}

	scraperRetryBackoffMillis, err := getEnvInt("PRICEALERT_SCRAPER_RETRY_BACKOFF_MS", defaultScraperRetryBackoffMS)
	if err != nil {
		return Config{}, err
	}

	telegramTimeoutSeconds, err := getEnvInt("PRICEALERT_TELEGRAM_TIMEOUT_SECONDS", defaultTelegramTimeoutSeconds)
	if err != nil {
		return Config{}, err
	}

	rawListingRetentionHours, err := getEnvInt("PRICEALERT_RAW_LISTING_RETENTION_HOURS", defaultRawListingRetentionHrs)
	if err != nil {
		return Config{}, err
	}

	alertEventRetentionHours, err := getEnvInt("PRICEALERT_ALERT_EVENT_RETENTION_HOURS", defaultAlertEventRetentionHrs)
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
		Scraper: ScraperConfig{
			TokopediaSearchEndpoint: getEnv("PRICEALERT_TOKOPEDIA_SEARCH_ENDPOINT", defaultTokopediaSearchEndpoint),
			TimeoutSeconds:          scraperTimeoutSeconds,
			RowsPerScan:             scraperRows,
			RetryAttempts:           scraperRetryAttempts,
			RetryBackoffMillis:      scraperRetryBackoffMillis,
		},
		Telegram: TelegramConfig{
			BotToken:       getEnv("PRICEALERT_TELEGRAM_BOT_TOKEN", ""),
			ChatID:         getEnv("PRICEALERT_TELEGRAM_CHAT_ID", ""),
			APIBaseURL:     getEnv("PRICEALERT_TELEGRAM_API_BASE_URL", defaultTelegramAPIBaseURL),
			TimeoutSeconds: telegramTimeoutSeconds,
		},
		Retention: RetentionConfig{
			RawListingsHours: rawListingRetentionHours,
			AlertEventsHours: alertEventRetentionHours,
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

	if c.Scraper.TokopediaSearchEndpoint == "" {
		return fmt.Errorf("tokopedia search endpoint is required")
	}

	if c.Scraper.TimeoutSeconds <= 0 {
		return fmt.Errorf("scraper timeout seconds must be > 0")
	}

	if c.Scraper.RowsPerScan <= 0 {
		return fmt.Errorf("scraper rows per scan must be > 0")
	}
	if c.Scraper.RetryAttempts <= 0 {
		return fmt.Errorf("scraper retry attempts must be > 0")
	}
	if c.Scraper.RetryBackoffMillis <= 0 {
		return fmt.Errorf("scraper retry backoff ms must be > 0")
	}

	if c.Telegram.TimeoutSeconds <= 0 {
		return fmt.Errorf("telegram timeout seconds must be > 0")
	}

	if (c.Telegram.BotToken == "") != (c.Telegram.ChatID == "") {
		return fmt.Errorf("telegram bot token and chat id must both be set or both be empty")
	}

	if c.Telegram.BotToken != "" && c.Telegram.APIBaseURL == "" {
		return fmt.Errorf("telegram api base url is required when telegram is configured")
	}

	if c.Retention.RawListingsHours < 0 {
		return fmt.Errorf("raw listing retention hours must be >= 0")
	}
	if c.Retention.AlertEventsHours < 0 {
		return fmt.Errorf("alert event retention hours must be >= 0")
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
