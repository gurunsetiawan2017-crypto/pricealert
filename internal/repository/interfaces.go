package repository

import (
	"context"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
)

type TrackedKeywordRepository interface {
	Create(context.Context, domain.TrackedKeyword) error
	Update(context.Context, domain.TrackedKeyword) error
	GetByID(context.Context, string) (*domain.TrackedKeyword, error)
	ListActive(context.Context) ([]domain.TrackedKeyword, error)
	ListVisible(context.Context) ([]domain.TrackedKeyword, error)
}

type ScanJobRepository interface {
	Create(context.Context, domain.ScanJob) error
	MarkSuccess(context.Context, string, int, int) error
	MarkFailed(context.Context, string, string) error
	GetLatestByKeywordID(context.Context, string) (*domain.ScanJob, error)
	ListRunning(context.Context, int) ([]domain.ScanJob, error)
}

type RawListingRepository interface {
	CreateBatch(context.Context, []domain.RawListing) error
	ListByScanJobID(context.Context, string) ([]domain.RawListing, error)
	PruneOlderThanScrapedAt(context.Context, time.Time) (int, error)
}

type GroupedListingRepository interface {
	CreateBatch(context.Context, []domain.GroupedListing) error
	ListByScanJobID(context.Context, string) ([]domain.GroupedListing, error)
}

type MarketSnapshotRepository interface {
	Create(context.Context, domain.MarketSnapshot) error
	GetLatestByKeywordID(context.Context, string) (*domain.MarketSnapshot, error)
}

type PricePointRepository interface {
	Create(context.Context, domain.PricePoint) error
	ListRecentByKeywordID(context.Context, string, int) ([]domain.PricePoint, error)
}

type AlertRuleRepository interface {
	Create(context.Context, domain.AlertRule) error
	ListEnabledByKeywordID(context.Context, string) ([]domain.AlertRule, error)
}

type AlertEventRepository interface {
	Create(context.Context, domain.AlertEvent) error
	MarkSentToTelegram(context.Context, string) error
	ListRecentByKeywordID(context.Context, string, int) ([]domain.AlertEvent, error)
	PruneOlderThanCreatedAt(context.Context, time.Time) (int, error)
}
