package query

import (
	"context"
	"database/sql"
	"errors"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/dto"
	"github.com/pricealert/pricealert/internal/repository"
)

const (
	defaultRecentEventsLimit  = 10
	defaultRecentHistoryLimit = 10
)

type Service struct {
	trackedKeywords repository.TrackedKeywordRepository
	groupedListings repository.GroupedListingRepository
	snapshots       repository.MarketSnapshotRepository
	pricePoints     repository.PricePointRepository
	alertEvents     repository.AlertEventRepository
	runtimeStatus   RuntimeStatusProvider
}

type RuntimeStatusProvider interface {
	Summary(context.Context) (*dto.RuntimeStatusSummary, error)
}

func NewService(
	trackedKeywords repository.TrackedKeywordRepository,
	groupedListings repository.GroupedListingRepository,
	snapshots repository.MarketSnapshotRepository,
	pricePoints repository.PricePointRepository,
	alertEvents repository.AlertEventRepository,
	runtimeStatus RuntimeStatusProvider,
) *Service {
	return &Service{
		trackedKeywords: trackedKeywords,
		groupedListings: groupedListings,
		snapshots:       snapshots,
		pricePoints:     pricePoints,
		alertEvents:     alertEvents,
		runtimeStatus:   runtimeStatus,
	}
}

func (s *Service) DashboardState(ctx context.Context, selectedKeywordID *string) (*dto.DashboardState, error) {
	keywords, err := s.trackedKeywords.ListVisible(ctx)
	if err != nil {
		return nil, err
	}

	state := &dto.DashboardState{
		TrackedKeywords: make([]dto.TrackedKeywordSummary, 0, len(keywords)),
		TopDeals:        []dto.GroupedListing{},
		RecentEvents:    []dto.AlertEvent{},
	}
	if s.runtimeStatus != nil {
		summary, err := s.runtimeStatus.Summary(ctx)
		if err != nil {
			return nil, err
		}
		state.RuntimeStatus = summary
	}
	if len(keywords) == 0 {
		return state, nil
	}

	for _, keyword := range keywords {
		events, err := s.alertEvents.ListRecentByKeywordID(ctx, keyword.ID, 1)
		if err != nil {
			return nil, err
		}
		state.TrackedKeywords = append(state.TrackedKeywords, dto.TrackedKeywordSummary{
			ID:          keyword.ID,
			Keyword:     keyword.Keyword,
			Status:      string(keyword.Status),
			HasNewAlert: hasNewAlert(events),
		})
	}

	selected := chooseSelectedKeyword(keywords, selectedKeywordID)
	if selected == nil {
		return state, nil
	}
	state.SelectedKeywordID = &selected.ID

	snapshot, err := s.snapshots.GetLatestByKeywordID(ctx, selected.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if snapshot == nil {
		return state, nil
	}

	state.SelectedSnapshot = mapSnapshot(*snapshot)

	topDeals, err := s.groupedListings.ListByScanJobID(ctx, snapshot.ScanJobID)
	if err != nil {
		return nil, err
	}
	state.TopDeals = mapGroupedListings(topDeals)

	recentEvents, err := s.alertEvents.ListRecentByKeywordID(ctx, selected.ID, defaultRecentEventsLimit)
	if err != nil {
		return nil, err
	}
	state.RecentEvents = mapAlertEvents(recentEvents)

	return state, nil
}

func (s *Service) KeywordDetail(ctx context.Context, keywordID string) (*dto.KeywordDetail, error) {
	keyword, err := s.trackedKeywords.GetByID(ctx, keywordID)
	if err != nil {
		return nil, err
	}

	detail := &dto.KeywordDetail{
		Keyword:       mapTrackedKeyword(*keyword),
		TopDeals:      []dto.GroupedListing{},
		RecentEvents:  []dto.AlertEvent{},
		RecentHistory: []dto.PricePoint{},
	}

	snapshot, err := s.snapshots.GetLatestByKeywordID(ctx, keywordID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if snapshot != nil {
		detail.Snapshot = mapSnapshot(*snapshot)

		topDeals, err := s.groupedListings.ListByScanJobID(ctx, snapshot.ScanJobID)
		if err != nil {
			return nil, err
		}
		detail.TopDeals = mapGroupedListings(topDeals)
	}

	recentEvents, err := s.alertEvents.ListRecentByKeywordID(ctx, keywordID, defaultRecentEventsLimit)
	if err != nil {
		return nil, err
	}
	detail.RecentEvents = mapAlertEvents(recentEvents)

	recentHistory, err := s.pricePoints.ListRecentByKeywordID(ctx, keywordID, defaultRecentHistoryLimit)
	if err != nil {
		return nil, err
	}
	detail.RecentHistory = mapPricePoints(recentHistory)

	return detail, nil
}

func chooseSelectedKeyword(keywords []domain.TrackedKeyword, selectedKeywordID *string) *domain.TrackedKeyword {
	if len(keywords) == 0 {
		return nil
	}

	if selectedKeywordID != nil {
		for _, keyword := range keywords {
			if keyword.ID == *selectedKeywordID {
				selected := keyword
				return &selected
			}
		}
	}

	selected := keywords[0]
	return &selected
}

func hasNewAlert(events []domain.AlertEvent) bool {
	if len(events) == 0 {
		return false
	}
	return events[0].Level == domain.AlertLevelAlert
}

func mapTrackedKeyword(keyword domain.TrackedKeyword) dto.TrackedKeyword {
	return dto.TrackedKeyword{
		ID:              keyword.ID,
		Keyword:         keyword.Keyword,
		BasicFilter:     keyword.BasicFilter,
		ThresholdPrice:  keyword.ThresholdPrice,
		IntervalMinutes: keyword.IntervalMinutes,
		TelegramEnabled: keyword.TelegramEnabled,
		Status:          string(keyword.Status),
		CreatedAt:       keyword.CreatedAt,
		UpdatedAt:       keyword.UpdatedAt,
	}
}

func mapSnapshot(snapshot domain.MarketSnapshot) *dto.MarketSnapshot {
	return &dto.MarketSnapshot{
		ID:               snapshot.ID,
		TrackedKeywordID: snapshot.TrackedKeywordID,
		ScanJobID:        snapshot.ScanJobID,
		MinPrice:         snapshot.MinPrice,
		AvgPrice:         snapshot.AvgPrice,
		MaxPrice:         snapshot.MaxPrice,
		RawCount:         snapshot.RawCount,
		GroupedCount:     snapshot.GroupedCount,
		Signal:           string(snapshot.Signal),
		SnapshotAt:       snapshot.SnapshotAt,
	}
}

func mapGroupedListings(listings []domain.GroupedListing) []dto.GroupedListing {
	if len(listings) == 0 {
		return []dto.GroupedListing{}
	}

	mapped := make([]dto.GroupedListing, 0, len(listings))
	for _, listing := range listings {
		mapped = append(mapped, dto.GroupedListing{
			ID:                   listing.ID,
			GroupKey:             listing.GroupKey,
			RepresentativeTitle:  listing.RepresentativeTitle,
			RepresentativeSeller: listing.RepresentativeSeller,
			BestPrice:            listing.BestPrice,
			OriginalPrice:        listing.OriginalPrice,
			IsPromo:              listing.IsPromo,
			ListingCount:         listing.ListingCount,
			SampleURL:            listing.SampleURL,
		})
	}

	return mapped
}

func mapAlertEvents(events []domain.AlertEvent) []dto.AlertEvent {
	if len(events) == 0 {
		return []dto.AlertEvent{}
	}

	mapped := make([]dto.AlertEvent, 0, len(events))
	for _, event := range events {
		mapped = append(mapped, dto.AlertEvent{
			ID:               event.ID,
			TrackedKeywordID: event.TrackedKeywordID,
			ScanJobID:        event.ScanJobID,
			Level:            string(event.Level),
			EventType:        string(event.EventType),
			Message:          event.Message,
			PayloadJSON:      event.PayloadJSON,
			SentToTelegram:   event.SentToTelegram,
			CreatedAt:        event.CreatedAt,
		})
	}

	return mapped
}

func mapPricePoints(points []domain.PricePoint) []dto.PricePoint {
	if len(points) == 0 {
		return []dto.PricePoint{}
	}

	mapped := make([]dto.PricePoint, 0, len(points))
	for _, point := range points {
		mapped = append(mapped, dto.PricePoint{
			ID:               point.ID,
			TrackedKeywordID: point.TrackedKeywordID,
			ScanJobID:        point.ScanJobID,
			MinPrice:         point.MinPrice,
			AvgPrice:         point.AvgPrice,
			MaxPrice:         point.MaxPrice,
			RecordedAt:       point.RecordedAt,
		})
	}

	return mapped
}
