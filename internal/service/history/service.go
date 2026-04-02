package history

import "github.com/pricealert/pricealert/internal/domain"

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) BuildFromSnapshot(snapshot domain.MarketSnapshot) domain.PricePoint {
	return domain.PricePoint{
		ID:               "",
		TrackedKeywordID: snapshot.TrackedKeywordID,
		ScanJobID:        snapshot.ScanJobID,
		MinPrice:         cloneInt64Pointer(snapshot.MinPrice),
		AvgPrice:         cloneInt64Pointer(snapshot.AvgPrice),
		MaxPrice:         cloneInt64Pointer(snapshot.MaxPrice),
		RecordedAt:       snapshot.SnapshotAt,
	}
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}

	v := *value
	return &v
}
