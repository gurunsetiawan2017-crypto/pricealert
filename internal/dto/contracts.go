package dto

import "time"

type TrackedKeyword struct {
	ID              string    `json:"id"`
	Keyword         string    `json:"keyword"`
	BasicFilter     *string   `json:"basic_filter"`
	ThresholdPrice  *int64    `json:"threshold_price"`
	IntervalMinutes int       `json:"interval_minutes"`
	TelegramEnabled bool      `json:"telegram_enabled"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TrackedKeywordSummary struct {
	ID          string `json:"id"`
	Keyword     string `json:"keyword"`
	Status      string `json:"status"`
	HasNewAlert bool   `json:"has_new_alert"`
}

type GroupedListing struct {
	ID                   string `json:"id"`
	GroupKey             string `json:"group_key"`
	RepresentativeTitle  string `json:"representative_title"`
	RepresentativeSeller string `json:"representative_seller"`
	BestPrice            int64  `json:"best_price"`
	OriginalPrice        *int64 `json:"original_price"`
	IsPromo              bool   `json:"is_promo"`
	ListingCount         int    `json:"listing_count"`
	SampleURL            string `json:"sample_url"`
}

type MarketSnapshot struct {
	ID               string    `json:"id"`
	TrackedKeywordID string    `json:"tracked_keyword_id"`
	ScanJobID        string    `json:"scan_job_id"`
	MinPrice         *int64    `json:"min_price"`
	AvgPrice         *int64    `json:"avg_price"`
	MaxPrice         *int64    `json:"max_price"`
	RawCount         int       `json:"raw_count"`
	GroupedCount     int       `json:"grouped_count"`
	Signal           string    `json:"signal"`
	SnapshotAt       time.Time `json:"snapshot_at"`
}

type AlertEvent struct {
	ID               string    `json:"id"`
	TrackedKeywordID string    `json:"tracked_keyword_id"`
	ScanJobID        *string   `json:"scan_job_id"`
	Level            string    `json:"level"`
	EventType        string    `json:"event_type"`
	Message          string    `json:"message"`
	PayloadJSON      *string   `json:"payload_json"`
	SentToTelegram   bool      `json:"sent_to_telegram"`
	CreatedAt        time.Time `json:"created_at"`
}

type PricePoint struct {
	ID               string    `json:"id"`
	TrackedKeywordID string    `json:"tracked_keyword_id"`
	ScanJobID        string    `json:"scan_job_id"`
	MinPrice         *int64    `json:"min_price"`
	AvgPrice         *int64    `json:"avg_price"`
	MaxPrice         *int64    `json:"max_price"`
	RecordedAt       time.Time `json:"recorded_at"`
}

type KeywordDetail struct {
	Keyword       TrackedKeyword   `json:"keyword"`
	Snapshot      *MarketSnapshot  `json:"snapshot"`
	TopDeals      []GroupedListing `json:"top_deals"`
	RecentEvents  []AlertEvent     `json:"recent_events"`
	RecentHistory []PricePoint     `json:"recent_history"`
}

type DashboardState struct {
	TrackedKeywords   []TrackedKeywordSummary `json:"tracked_keywords"`
	SelectedKeywordID *string                 `json:"selected_keyword_id"`
	SelectedSnapshot  *MarketSnapshot         `json:"selected_snapshot"`
	TopDeals          []GroupedListing        `json:"top_deals"`
	RecentEvents      []AlertEvent            `json:"recent_events"`
}
