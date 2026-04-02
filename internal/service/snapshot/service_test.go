package snapshot

import (
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/service/history"
)

func TestBuildNoDataSnapshot(t *testing.T) {
	service := NewService()
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)

	got := service.Build("kw_1", "scan_1", 5, nil, snapshotAt)

	if got.TrackedKeywordID != "kw_1" {
		t.Fatalf("tracked keyword id = %q, want %q", got.TrackedKeywordID, "kw_1")
	}
	if got.ScanJobID != "scan_1" {
		t.Fatalf("scan job id = %q, want %q", got.ScanJobID, "scan_1")
	}
	if got.RawCount != 5 {
		t.Fatalf("raw count = %d, want %d", got.RawCount, 5)
	}
	if got.GroupedCount != 0 {
		t.Fatalf("grouped count = %d, want %d", got.GroupedCount, 0)
	}
	if got.MinPrice != nil || got.AvgPrice != nil || got.MaxPrice != nil {
		t.Fatalf("expected nil prices for no-data snapshot")
	}
	if got.Signal != domain.MarketSignalNoData {
		t.Fatalf("signal = %q, want %q", got.Signal, domain.MarketSignalNoData)
	}
}

func TestBuildSingleGroupedListingSnapshot(t *testing.T) {
	service := NewService()
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)

	got := service.Build("kw_1", "scan_1", 4, []domain.GroupedListing{
		{BestPrice: 23800},
	}, snapshotAt)

	assertPrice(t, got.MinPrice, 23800, "min")
	assertPrice(t, got.AvgPrice, 23800, "avg")
	assertPrice(t, got.MaxPrice, 23800, "max")

	if got.GroupedCount != 1 {
		t.Fatalf("grouped count = %d, want %d", got.GroupedCount, 1)
	}
	if got.Signal != domain.MarketSignalNormal {
		t.Fatalf("signal = %q, want %q", got.Signal, domain.MarketSignalNormal)
	}
}

func TestBuildMultipleGroupedListingsSnapshot(t *testing.T) {
	service := NewService()
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)

	got := service.Build("kw_1", "scan_1", 9, []domain.GroupedListing{
		{BestPrice: 22000},
		{BestPrice: 25000},
		{BestPrice: 31000},
	}, snapshotAt)

	assertPrice(t, got.MinPrice, 22000, "min")
	assertPrice(t, got.AvgPrice, 26000, "avg")
	assertPrice(t, got.MaxPrice, 31000, "max")

	if got.RawCount != 9 {
		t.Fatalf("raw count = %d, want %d", got.RawCount, 9)
	}
	if got.GroupedCount != 3 {
		t.Fatalf("grouped count = %d, want %d", got.GroupedCount, 3)
	}
	if got.Signal != domain.MarketSignalGoodDeal {
		t.Fatalf("signal = %q, want %q", got.Signal, domain.MarketSignalGoodDeal)
	}
}

func TestGroupedCountIsIndependentFromRawCount(t *testing.T) {
	service := NewService()
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)

	got := service.Build("kw_1", "scan_1", 20, []domain.GroupedListing{
		{BestPrice: 21000, ListingCount: 7},
		{BestPrice: 24000, ListingCount: 6},
		{BestPrice: 26000, ListingCount: 7},
	}, snapshotAt)

	if got.RawCount != 20 {
		t.Fatalf("raw count = %d, want %d", got.RawCount, 20)
	}
	if got.GroupedCount != 3 {
		t.Fatalf("grouped count = %d, want %d", got.GroupedCount, 3)
	}
}

func TestSignalAssignmentBehavior(t *testing.T) {
	service := NewService()
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)

	tests := []struct {
		name     string
		prices   []int64
		expected domain.MarketSignal
	}{
		{
			name:     "normal when grouped count below threshold",
			prices:   []int64{20000, 24000},
			expected: domain.MarketSignalNormal,
		},
		{
			name:     "good deal when min is meaningfully below grouped average",
			prices:   []int64{21000, 24000, 27000},
			expected: domain.MarketSignalGoodDeal,
		},
		{
			name:     "buy now when min is deeply below grouped average",
			prices:   []int64{15000, 24000, 27000, 30000},
			expected: domain.MarketSignalBuyNow,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			grouped := make([]domain.GroupedListing, 0, len(tc.prices))
			for _, price := range tc.prices {
				grouped = append(grouped, domain.GroupedListing{BestPrice: price})
			}

			got := service.Build("kw_1", "scan_1", len(tc.prices), grouped, snapshotAt)
			if got.Signal != tc.expected {
				t.Fatalf("signal = %q, want %q", got.Signal, tc.expected)
			}
		})
	}
}

func TestBuildPricePointFromSnapshot(t *testing.T) {
	historyService := history.NewService()
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)
	minPrice := int64(22000)
	avgPrice := int64(25000)
	maxPrice := int64(28000)

	point := historyService.BuildFromSnapshot(domain.MarketSnapshot{
		TrackedKeywordID: "kw_1",
		ScanJobID:        "scan_1",
		MinPrice:         &minPrice,
		AvgPrice:         &avgPrice,
		MaxPrice:         &maxPrice,
		SnapshotAt:       snapshotAt,
	})

	if point.TrackedKeywordID != "kw_1" {
		t.Fatalf("tracked keyword id = %q, want %q", point.TrackedKeywordID, "kw_1")
	}
	if point.ScanJobID != "scan_1" {
		t.Fatalf("scan job id = %q, want %q", point.ScanJobID, "scan_1")
	}
	assertPrice(t, point.MinPrice, 22000, "min")
	assertPrice(t, point.AvgPrice, 25000, "avg")
	assertPrice(t, point.MaxPrice, 28000, "max")
	if !point.RecordedAt.Equal(snapshotAt) {
		t.Fatalf("recorded_at = %v, want %v", point.RecordedAt, snapshotAt)
	}
}

func assertPrice(t *testing.T, got *int64, want int64, label string) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s price is nil, want %d", label, want)
	}
	if *got != want {
		t.Fatalf("%s price = %d, want %d", label, *got, want)
	}
}
