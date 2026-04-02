package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
)

const (
	defaultThresholdCooldown     = time.Hour
	defaultNewLowestCooldown     = time.Hour
	defaultMeaningfulImprovement = 0.03
	defaultMinHistoryPoints      = 1
)

type Config struct {
	ThresholdCooldown        time.Duration
	NewLowestCooldown        time.Duration
	MeaningfulImprovementPct float64
	MinHistoryPoints         int
}

type Service struct {
	config Config
}

type eventPayload struct {
	CurrentMinPrice *int64 `json:"current_min_price,omitempty"`
	ThresholdPrice  *int64 `json:"threshold_price,omitempty"`
	HistoryLowest   *int64 `json:"history_lowest,omitempty"`
}

func NewService() *Service {
	return NewServiceWithConfig(Config{
		ThresholdCooldown:        defaultThresholdCooldown,
		NewLowestCooldown:        defaultNewLowestCooldown,
		MeaningfulImprovementPct: defaultMeaningfulImprovement,
		MinHistoryPoints:         defaultMinHistoryPoints,
	})
}

func NewServiceWithConfig(config Config) *Service {
	if config.ThresholdCooldown <= 0 {
		config.ThresholdCooldown = defaultThresholdCooldown
	}
	if config.NewLowestCooldown <= 0 {
		config.NewLowestCooldown = defaultNewLowestCooldown
	}
	if config.MeaningfulImprovementPct <= 0 {
		config.MeaningfulImprovementPct = defaultMeaningfulImprovement
	}
	if config.MinHistoryPoints <= 0 {
		config.MinHistoryPoints = defaultMinHistoryPoints
	}

	return &Service{config: config}
}

func (s *Service) Evaluate(keyword domain.TrackedKeyword, snapshot domain.MarketSnapshot, history []domain.PricePoint, recentEvents []domain.AlertEvent) []domain.AlertEvent {
	if snapshot.MinPrice == nil {
		return []domain.AlertEvent{}
	}

	events := make([]domain.AlertEvent, 0, 2)

	if event, ok := s.evaluateThresholdHit(keyword, snapshot, recentEvents); ok {
		events = append(events, event)
	}

	if event, ok := s.evaluateNewLowest(keyword, snapshot, history, recentEvents); ok {
		events = append(events, event)
	}

	return events
}

func (s *Service) evaluateThresholdHit(keyword domain.TrackedKeyword, snapshot domain.MarketSnapshot, recentEvents []domain.AlertEvent) (domain.AlertEvent, bool) {
	if keyword.ThresholdPrice == nil || snapshot.MinPrice == nil {
		return domain.AlertEvent{}, false
	}

	if *snapshot.MinPrice > *keyword.ThresholdPrice {
		return domain.AlertEvent{}, false
	}

	lastEvent, found := latestAlertEvent(recentEvents, domain.AlertEventTypeThresholdHit)
	if found && s.blockRepeatedAlert(*snapshot.MinPrice, snapshot.SnapshotAt, lastEvent, s.config.ThresholdCooldown) {
		return domain.AlertEvent{}, false
	}

	payload, err := marshalPayload(eventPayload{
		CurrentMinPrice: snapshot.MinPrice,
		ThresholdPrice:  keyword.ThresholdPrice,
	})
	if err != nil {
		return domain.AlertEvent{}, false
	}

	return buildAlertEvent(
		keyword,
		snapshot,
		domain.AlertEventTypeThresholdHit,
		fmt.Sprintf("%s hit threshold: current grouped low %d is at or below threshold %d", keyword.Keyword, *snapshot.MinPrice, *keyword.ThresholdPrice),
		payload,
	), true
}

func (s *Service) evaluateNewLowest(keyword domain.TrackedKeyword, snapshot domain.MarketSnapshot, history []domain.PricePoint, recentEvents []domain.AlertEvent) (domain.AlertEvent, bool) {
	if snapshot.MinPrice == nil {
		return domain.AlertEvent{}, false
	}

	validHistory := priorValidHistory(history, snapshot.SnapshotAt)
	if len(validHistory) < s.config.MinHistoryPoints {
		return domain.AlertEvent{}, false
	}

	historyLowest := lowestHistoricalMin(validHistory)
	if historyLowest == nil || *snapshot.MinPrice >= *historyLowest {
		return domain.AlertEvent{}, false
	}

	lastEvent, found := latestAlertEvent(recentEvents, domain.AlertEventTypeNewLowest)
	if found && s.blockRepeatedAlert(*snapshot.MinPrice, snapshot.SnapshotAt, lastEvent, s.config.NewLowestCooldown) {
		return domain.AlertEvent{}, false
	}

	payload, err := marshalPayload(eventPayload{
		CurrentMinPrice: snapshot.MinPrice,
		HistoryLowest:   historyLowest,
	})
	if err != nil {
		return domain.AlertEvent{}, false
	}

	return buildAlertEvent(
		keyword,
		snapshot,
		domain.AlertEventTypeNewLowest,
		fmt.Sprintf("%s hit new lowest observed grouped price: %d", keyword.Keyword, *snapshot.MinPrice),
		payload,
	), true
}

func buildAlertEvent(keyword domain.TrackedKeyword, snapshot domain.MarketSnapshot, eventType domain.AlertEventType, message string, payload *string) domain.AlertEvent {
	scanJobID := snapshot.ScanJobID

	return domain.AlertEvent{
		ID:               "",
		TrackedKeywordID: keyword.ID,
		ScanJobID:        &scanJobID,
		Level:            domain.AlertLevelAlert,
		EventType:        eventType,
		Message:          message,
		PayloadJSON:      payload,
		SentToTelegram:   false,
		CreatedAt:        snapshot.SnapshotAt,
	}
}

func (s *Service) blockRepeatedAlert(currentPrice int64, now time.Time, lastEvent domain.AlertEvent, cooldown time.Duration) bool {
	lastPrice, ok := extractCurrentMinPrice(lastEvent)
	if ok {
		if currentPrice >= lastPrice {
			return true
		}

		if currentPrice == lastPrice {
			return true
		}

		if !meaningfulImprovement(currentPrice, lastPrice, s.config.MeaningfulImprovementPct) {
			return true
		}
	}

	if !lastEvent.CreatedAt.IsZero() && now.Sub(lastEvent.CreatedAt) < cooldown {
		return true
	}

	return false
}

func latestAlertEvent(events []domain.AlertEvent, eventType domain.AlertEventType) (domain.AlertEvent, bool) {
	var latest domain.AlertEvent
	found := false

	for _, event := range events {
		if event.EventType != eventType {
			continue
		}

		if !found || event.CreatedAt.After(latest.CreatedAt) {
			latest = event
			found = true
		}
	}

	return latest, found
}

func priorValidHistory(history []domain.PricePoint, snapshotAt time.Time) []domain.PricePoint {
	valid := make([]domain.PricePoint, 0, len(history))

	for _, point := range history {
		if point.MinPrice == nil {
			continue
		}

		if !point.RecordedAt.Before(snapshotAt) {
			continue
		}

		valid = append(valid, point)
	}

	return valid
}

func lowestHistoricalMin(history []domain.PricePoint) *int64 {
	if len(history) == 0 {
		return nil
	}

	lowest := *history[0].MinPrice
	for _, point := range history[1:] {
		if point.MinPrice != nil && *point.MinPrice < lowest {
			lowest = *point.MinPrice
		}
	}

	return &lowest
}

func extractCurrentMinPrice(event domain.AlertEvent) (int64, bool) {
	if event.PayloadJSON == nil || *event.PayloadJSON == "" {
		return 0, false
	}

	var payload eventPayload
	if err := json.Unmarshal([]byte(*event.PayloadJSON), &payload); err != nil {
		return 0, false
	}

	if payload.CurrentMinPrice == nil {
		return 0, false
	}

	return *payload.CurrentMinPrice, true
}

func meaningfulImprovement(currentPrice, previousPrice int64, minimumPct float64) bool {
	if currentPrice >= previousPrice || previousPrice <= 0 {
		return false
	}

	improvement := float64(previousPrice-currentPrice) / float64(previousPrice)
	return improvement >= minimumPct
}

func marshalPayload(payload eventPayload) (*string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	value := string(raw)
	return &value, nil
}
