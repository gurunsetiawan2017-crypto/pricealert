package alert

import (
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
)

func TestThresholdHitTriggersWhenMinPriceAtOrBelowThreshold(t *testing.T) {
	service := NewServiceWithConfig(Config{
		ThresholdCooldown:        time.Hour,
		NewLowestCooldown:        time.Hour,
		MeaningfulImprovementPct: 0.03,
		MinHistoryPoints:         1,
	})

	threshold := int64(25000)
	minPrice := int64(23800)
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)

	events := service.Evaluate(
		domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L", ThresholdPrice: &threshold},
		domain.MarketSnapshot{TrackedKeywordID: "kw_1", ScanJobID: "scan_1", MinPrice: &minPrice, SnapshotAt: snapshotAt},
		nil,
		nil,
	)

	if len(events) != 1 {
		t.Fatalf("event count = %d, want %d", len(events), 1)
	}
	if events[0].EventType != domain.AlertEventTypeThresholdHit {
		t.Fatalf("event type = %q, want %q", events[0].EventType, domain.AlertEventTypeThresholdHit)
	}
	if events[0].Level != domain.AlertLevelAlert {
		t.Fatalf("level = %q, want %q", events[0].Level, domain.AlertLevelAlert)
	}
	if events[0].PayloadJSON == nil {
		t.Fatalf("payload json is nil")
	}
}

func TestThresholdHitDoesNotSpamRepeatedlyAtSamePrice(t *testing.T) {
	service := NewServiceWithConfig(Config{
		ThresholdCooldown:        time.Hour,
		NewLowestCooldown:        time.Hour,
		MeaningfulImprovementPct: 0.03,
		MinHistoryPoints:         1,
	})

	threshold := int64(25000)
	minPrice := int64(23800)
	snapshotAt := time.Date(2026, 4, 2, 11, 0, 0, 0, time.UTC)
	lastPayload := payloadJSON(t, eventPayload{
		CurrentMinPrice: &minPrice,
		ThresholdPrice:  &threshold,
	})

	events := service.Evaluate(
		domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L", ThresholdPrice: &threshold},
		domain.MarketSnapshot{TrackedKeywordID: "kw_1", ScanJobID: "scan_2", MinPrice: &minPrice, SnapshotAt: snapshotAt},
		nil,
		[]domain.AlertEvent{
			{
				EventType:   domain.AlertEventTypeThresholdHit,
				PayloadJSON: &lastPayload,
				CreatedAt:   snapshotAt.Add(-30 * time.Minute),
			},
		},
	)

	if len(events) != 0 {
		t.Fatalf("event count = %d, want %d", len(events), 0)
	}
}

func TestNewLowestTriggersOnlyWhenLowerThanPriorHistory(t *testing.T) {
	service := NewServiceWithConfig(Config{
		ThresholdCooldown:        time.Hour,
		NewLowestCooldown:        time.Hour,
		MeaningfulImprovementPct: 0.03,
		MinHistoryPoints:         1,
	})

	minPrice := int64(23000)
	priorLowest := int64(24000)
	priorHigher := int64(26000)
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)

	events := service.Evaluate(
		domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L"},
		domain.MarketSnapshot{TrackedKeywordID: "kw_1", ScanJobID: "scan_1", MinPrice: &minPrice, SnapshotAt: snapshotAt},
		[]domain.PricePoint{
			{MinPrice: &priorLowest, RecordedAt: snapshotAt.Add(-2 * time.Hour)},
			{MinPrice: &priorHigher, RecordedAt: snapshotAt.Add(-time.Hour)},
		},
		nil,
	)

	if len(events) != 1 {
		t.Fatalf("event count = %d, want %d", len(events), 1)
	}
	if events[0].EventType != domain.AlertEventTypeNewLowest {
		t.Fatalf("event type = %q, want %q", events[0].EventType, domain.AlertEventTypeNewLowest)
	}
}

func TestNewLowestDoesNotFireWhenHistoryIsInsufficient(t *testing.T) {
	service := NewServiceWithConfig(Config{
		ThresholdCooldown:        time.Hour,
		NewLowestCooldown:        time.Hour,
		MeaningfulImprovementPct: 0.03,
		MinHistoryPoints:         1,
	})

	minPrice := int64(23000)
	snapshotAt := time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)

	events := service.Evaluate(
		domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L"},
		domain.MarketSnapshot{TrackedKeywordID: "kw_1", ScanJobID: "scan_1", MinPrice: &minPrice, SnapshotAt: snapshotAt},
		nil,
		nil,
	)

	if len(events) != 0 {
		t.Fatalf("event count = %d, want %d", len(events), 0)
	}
}

func TestMeaningfulImprovementRequiredForRepeatedAlert(t *testing.T) {
	service := NewServiceWithConfig(Config{
		ThresholdCooldown:        time.Minute,
		NewLowestCooldown:        time.Minute,
		MeaningfulImprovementPct: 0.05,
		MinHistoryPoints:         1,
	})

	threshold := int64(25000)
	lastPrice := int64(24000)
	currentPrice := int64(23850)
	snapshotAt := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	lastPayload := payloadJSON(t, eventPayload{
		CurrentMinPrice: &lastPrice,
		ThresholdPrice:  &threshold,
	})

	events := service.Evaluate(
		domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L", ThresholdPrice: &threshold},
		domain.MarketSnapshot{TrackedKeywordID: "kw_1", ScanJobID: "scan_2", MinPrice: &currentPrice, SnapshotAt: snapshotAt},
		nil,
		[]domain.AlertEvent{
			{
				EventType:   domain.AlertEventTypeThresholdHit,
				PayloadJSON: &lastPayload,
				CreatedAt:   snapshotAt.Add(-2 * time.Hour),
			},
		},
	)

	if len(events) != 0 {
		t.Fatalf("event count = %d, want %d", len(events), 0)
	}
}

func TestNoAlertWhenSnapshotHasNoUsableData(t *testing.T) {
	service := NewService()
	threshold := int64(25000)

	events := service.Evaluate(
		domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L", ThresholdPrice: &threshold},
		domain.MarketSnapshot{TrackedKeywordID: "kw_1", ScanJobID: "scan_1"},
		nil,
		nil,
	)

	if len(events) != 0 {
		t.Fatalf("event count = %d, want %d", len(events), 0)
	}
}

func TestCooldownBlocksRepeatedAlertEvenWhenPriceImproves(t *testing.T) {
	service := NewServiceWithConfig(Config{
		ThresholdCooldown:        time.Hour,
		NewLowestCooldown:        time.Hour,
		MeaningfulImprovementPct: 0.03,
		MinHistoryPoints:         1,
	})

	threshold := int64(25000)
	lastPrice := int64(24000)
	currentPrice := int64(23000)
	snapshotAt := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	lastPayload := payloadJSON(t, eventPayload{
		CurrentMinPrice: &lastPrice,
		ThresholdPrice:  &threshold,
	})

	events := service.Evaluate(
		domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L", ThresholdPrice: &threshold},
		domain.MarketSnapshot{TrackedKeywordID: "kw_1", ScanJobID: "scan_2", MinPrice: &currentPrice, SnapshotAt: snapshotAt},
		nil,
		[]domain.AlertEvent{
			{
				EventType:   domain.AlertEventTypeThresholdHit,
				PayloadJSON: &lastPayload,
				CreatedAt:   snapshotAt.Add(-30 * time.Minute),
			},
		},
	)

	if len(events) != 0 {
		t.Fatalf("event count = %d, want %d", len(events), 0)
	}
}

func payloadJSON(t *testing.T, payload eventPayload) string {
	t.Helper()

	raw, err := marshalPayload(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return *raw
}
