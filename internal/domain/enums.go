package domain

type TrackedKeywordStatus string

const (
	TrackedKeywordStatusActive   TrackedKeywordStatus = "active"
	TrackedKeywordStatusPaused   TrackedKeywordStatus = "paused"
	TrackedKeywordStatusArchived TrackedKeywordStatus = "archived"
)

type ScanJobStatus string

const (
	ScanJobStatusRunning ScanJobStatus = "running"
	ScanJobStatusSuccess ScanJobStatus = "success"
	ScanJobStatusFailed  ScanJobStatus = "failed"
)

type MarketSignal string

const (
	MarketSignalBuyNow   MarketSignal = "BUY_NOW"
	MarketSignalGoodDeal MarketSignal = "GOOD_DEAL"
	MarketSignalNormal   MarketSignal = "NORMAL"
	MarketSignalNoData   MarketSignal = "NO_DATA"
)

type AlertLevel string

const (
	AlertLevelInfo  AlertLevel = "INFO"
	AlertLevelAlert AlertLevel = "ALERT"
	AlertLevelWarn  AlertLevel = "WARN"
	AlertLevelError AlertLevel = "ERROR"
)

type AlertRuleType string

const (
	AlertRuleTypeThresholdBelow AlertRuleType = "threshold_below"
	AlertRuleTypeNewLowest      AlertRuleType = "new_lowest"
	AlertRuleTypePriceDropPct   AlertRuleType = "price_drop_percent"
)

type AlertEventType string

const (
	AlertEventTypeScanStarted   AlertEventType = "scan_started"
	AlertEventTypeScanCompleted AlertEventType = "scan_completed"
	AlertEventTypeScanFailed    AlertEventType = "scan_failed"
	AlertEventTypeThresholdHit  AlertEventType = "threshold_hit"
	AlertEventTypeNewLowest     AlertEventType = "new_lowest"
	AlertEventTypePriceDrop     AlertEventType = "price_drop"
	AlertEventTypeTelegramSent  AlertEventType = "telegram_sent"
	AlertEventTypeTelegramFail  AlertEventType = "telegram_failed"
)
