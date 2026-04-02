package snapshot

import (
	"time"

	"github.com/pricealert/pricealert/internal/domain"
)

const (
	minGroupsForSignal      = 3
	buyNowDiscountThreshold = 0.25
	goodDealThreshold       = 0.10
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Build(trackedKeywordID, scanJobID string, rawCount int, groupedListings []domain.GroupedListing, snapshotAt time.Time) domain.MarketSnapshot {
	groupedCount := len(groupedListings)

	minPrice, avgPrice, maxPrice := summarizePrices(groupedListings)

	return domain.MarketSnapshot{
		ID:               "",
		TrackedKeywordID: trackedKeywordID,
		ScanJobID:        scanJobID,
		MinPrice:         minPrice,
		AvgPrice:         avgPrice,
		MaxPrice:         maxPrice,
		RawCount:         rawCount,
		GroupedCount:     groupedCount,
		Signal:           assignSignal(minPrice, avgPrice, groupedCount),
		SnapshotAt:       snapshotAt,
	}
}

func summarizePrices(groupedListings []domain.GroupedListing) (*int64, *int64, *int64) {
	if len(groupedListings) == 0 {
		return nil, nil, nil
	}

	minPrice := groupedListings[0].BestPrice
	maxPrice := groupedListings[0].BestPrice
	var sum int64

	for _, listing := range groupedListings {
		if listing.BestPrice < minPrice {
			minPrice = listing.BestPrice
		}

		if listing.BestPrice > maxPrice {
			maxPrice = listing.BestPrice
		}

		sum += listing.BestPrice
	}

	avgPrice := sum / int64(len(groupedListings))
	return int64Pointer(minPrice), int64Pointer(avgPrice), int64Pointer(maxPrice)
}

func assignSignal(minPrice, avgPrice *int64, groupedCount int) domain.MarketSignal {
	if minPrice == nil || avgPrice == nil || groupedCount == 0 {
		return domain.MarketSignalNoData
	}

	if groupedCount < minGroupsForSignal || *avgPrice <= 0 {
		return domain.MarketSignalNormal
	}

	discountRatio := float64(*avgPrice-*minPrice) / float64(*avgPrice)

	if discountRatio >= buyNowDiscountThreshold {
		return domain.MarketSignalBuyNow
	}

	if discountRatio >= goodDealThreshold {
		return domain.MarketSignalGoodDeal
	}

	return domain.MarketSignalNormal
}

func int64Pointer(value int64) *int64 {
	v := value
	return &v
}
