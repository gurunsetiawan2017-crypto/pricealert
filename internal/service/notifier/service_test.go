package notifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/dto"
)

func TestDispatchActionableMarksSentForEnabledKeyword(t *testing.T) {
	sender := &fakeSender{}
	repo := &fakeAlertEventRepo{}
	service := NewService(sender, fakeIDGen{id: "evt_fail_1"}, fakeClock{now: fixedNow()}, repo)

	minPrice := int64(23800)
	threshold := int64(25000)
	service.DispatchActionable(
		context.Background(),
		domain.TrackedKeyword{
			ID:              "kw_1",
			Keyword:         "minyak goreng 2L",
			ThresholdPrice:  &threshold,
			TelegramEnabled: true,
		},
		domain.MarketSnapshot{
			TrackedKeywordID: "kw_1",
			ScanJobID:        "scan_1",
			MinPrice:         &minPrice,
			Signal:           domain.MarketSignalBuyNow,
			SnapshotAt:       fixedNow(),
		},
		[]domain.GroupedListing{
			{
				RepresentativeTitle:  "Minyak Goreng 2L Promo",
				RepresentativeSeller: "Seller A",
				BestPrice:            23800,
				SampleURL:            "https://example.com/item/1",
				ListingCount:         2,
			},
		},
		[]domain.AlertEvent{
			{
				ID:               "evt_1",
				TrackedKeywordID: "kw_1",
				Level:            domain.AlertLevelAlert,
				EventType:        domain.AlertEventTypeThresholdHit,
				Message:          "threshold hit",
			},
		},
	)

	if len(sender.payloads) != 1 {
		t.Fatalf("payload count = %d, want 1", len(sender.payloads))
	}
	if sender.payloads[0].Keyword != "minyak goreng 2L" {
		t.Fatalf("keyword = %q", sender.payloads[0].Keyword)
	}
	if repo.markedSentEventID != "evt_1" {
		t.Fatalf("marked sent event id = %q", repo.markedSentEventID)
	}
	if len(repo.created) != 0 {
		t.Fatalf("unexpected created failure events = %d", len(repo.created))
	}
}

func TestDispatchActionableCreatesFailureEventOnSendError(t *testing.T) {
	sender := &fakeSender{err: errors.New("network down")}
	repo := &fakeAlertEventRepo{}
	service := NewService(sender, fakeIDGen{id: "evt_fail_1"}, fakeClock{now: fixedNow()}, repo)

	minPrice := int64(23800)
	service.DispatchActionable(
		context.Background(),
		domain.TrackedKeyword{
			ID:              "kw_1",
			Keyword:         "minyak goreng 2L",
			TelegramEnabled: true,
		},
		domain.MarketSnapshot{
			TrackedKeywordID: "kw_1",
			ScanJobID:        "scan_1",
			MinPrice:         &minPrice,
			Signal:           domain.MarketSignalGoodDeal,
			SnapshotAt:       fixedNow(),
		},
		nil,
		[]domain.AlertEvent{
			{
				ID:               "evt_1",
				TrackedKeywordID: "kw_1",
				Level:            domain.AlertLevelAlert,
				EventType:        domain.AlertEventTypeNewLowest,
				Message:          "new lowest",
			},
		},
	)

	if repo.markedSentEventID != "" {
		t.Fatalf("unexpected marked sent id = %q", repo.markedSentEventID)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created failure events = %d, want 1", len(repo.created))
	}
	if repo.created[0].EventType != domain.AlertEventTypeTelegramFail {
		t.Fatalf("event type = %q", repo.created[0].EventType)
	}
}

func TestDispatchActionableSkipsDisabledOrNonActionable(t *testing.T) {
	sender := &fakeSender{}
	repo := &fakeAlertEventRepo{}
	service := NewService(sender, fakeIDGen{id: "evt_fail_1"}, fakeClock{now: fixedNow()}, repo)

	service.DispatchActionable(
		context.Background(),
		domain.TrackedKeyword{ID: "kw_1", Keyword: "minyak goreng 2L", TelegramEnabled: false},
		domain.MarketSnapshot{TrackedKeywordID: "kw_1", ScanJobID: "scan_1", Signal: domain.MarketSignalNormal, SnapshotAt: fixedNow()},
		nil,
		[]domain.AlertEvent{
			{
				ID:               "evt_1",
				TrackedKeywordID: "kw_1",
				Level:            domain.AlertLevelWarn,
				EventType:        domain.AlertEventTypeTelegramFail,
				Message:          "ops",
			},
		},
	)

	if len(sender.payloads) != 0 {
		t.Fatalf("unexpected payloads = %d", len(sender.payloads))
	}
	if repo.markedSentEventID != "" || len(repo.created) != 0 {
		t.Fatalf("unexpected repo activity")
	}
}

type fakeSender struct {
	payloads []dto.TelegramAlertPayload
	err      error
}

func (f *fakeSender) SendAlert(_ context.Context, payload dto.TelegramAlertPayload) error {
	f.payloads = append(f.payloads, payload)
	return f.err
}

type fakeAlertEventRepo struct {
	created           []domain.AlertEvent
	markedSentEventID string
}

func (f *fakeAlertEventRepo) Create(_ context.Context, event domain.AlertEvent) error {
	f.created = append(f.created, event)
	return nil
}

func (f *fakeAlertEventRepo) MarkSentToTelegram(_ context.Context, eventID string) error {
	f.markedSentEventID = eventID
	return nil
}

func (f *fakeAlertEventRepo) ListRecentByKeywordID(context.Context, string, int) ([]domain.AlertEvent, error) {
	return nil, nil
}

func (f *fakeAlertEventRepo) PruneOlderThanCreatedAt(context.Context, time.Time) (int, error) {
	return 0, nil
}

type fakeIDGen struct {
	id string
}

func (f fakeIDGen) Next() string {
	return f.id
}

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}

func fixedNow() time.Time {
	return time.Date(2026, 4, 2, 10, 40, 0, 0, time.UTC)
}
