package domain

import "time"

type TrackedKeyword struct {
	ID              string
	Keyword         string
	BasicFilter     *string
	ThresholdPrice  *int64
	IntervalMinutes int
	TelegramEnabled bool
	Status          TrackedKeywordStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ScanJob struct {
	ID               string
	TrackedKeywordID string
	StartedAt        time.Time
	FinishedAt       *time.Time
	Status           ScanJobStatus
	ErrorMessage     *string
	RawCount         int
	GroupedCount     int
}

type RawListing struct {
	ID              string
	ScanJobID       string
	Source          string
	Title           string
	NormalizedTitle string
	SellerName      string
	Price           int64
	OriginalPrice   *int64
	IsPromo         bool
	URL             string
	ScrapedAt       time.Time
}

type GroupedListing struct {
	ID                   string
	ScanJobID            string
	GroupKey             string
	RepresentativeTitle  string
	RepresentativeSeller string
	BestPrice            int64
	OriginalPrice        *int64
	IsPromo              bool
	ListingCount         int
	SampleURL            string
}

type MarketSnapshot struct {
	ID               string
	TrackedKeywordID string
	ScanJobID        string
	MinPrice         *int64
	AvgPrice         *int64
	MaxPrice         *int64
	RawCount         int
	GroupedCount     int
	Signal           MarketSignal
	SnapshotAt       time.Time
}

type PricePoint struct {
	ID               string
	TrackedKeywordID string
	ScanJobID        string
	MinPrice         *int64
	AvgPrice         *int64
	MaxPrice         *int64
	RecordedAt       time.Time
}

type AlertRule struct {
	ID               string
	TrackedKeywordID string
	RuleType         AlertRuleType
	Value            string
	Enabled          bool
	CreatedAt        time.Time
}

type AlertEvent struct {
	ID               string
	TrackedKeywordID string
	ScanJobID        *string
	Level            AlertLevel
	EventType        AlertEventType
	Message          string
	PayloadJSON      *string
	SentToTelegram   bool
	CreatedAt        time.Time
}
