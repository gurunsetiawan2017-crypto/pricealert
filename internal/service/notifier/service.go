package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pricealert/pricealert/internal/domain"
	"github.com/pricealert/pricealert/internal/dto"
	"github.com/pricealert/pricealert/internal/repository"
)

type IDGenerator interface {
	Next() string
}

type Clock interface {
	Now() time.Time
}

type Sender interface {
	SendAlert(context.Context, dto.TelegramAlertPayload) error
}

type Service struct {
	sender      Sender
	idGenerator IDGenerator
	clock       Clock
	alertEvents repository.AlertEventRepository
}

func NewService(sender Sender, idGenerator IDGenerator, clock Clock, alertEvents repository.AlertEventRepository) *Service {
	return &Service{
		sender:      sender,
		idGenerator: idGenerator,
		clock:       clock,
		alertEvents: alertEvents,
	}
}

func (s *Service) DispatchActionable(
	ctx context.Context,
	keyword domain.TrackedKeyword,
	snapshot domain.MarketSnapshot,
	grouped []domain.GroupedListing,
	alerts []domain.AlertEvent,
) {
	if !keyword.TelegramEnabled || s.sender == nil {
		return
	}

	for _, event := range alerts {
		if !isActionableAlert(event) {
			continue
		}

		payload := buildTelegramPayload(keyword, snapshot, grouped, event)
		if err := s.sender.SendAlert(ctx, payload); err != nil {
			_ = s.alertEvents.Create(ctx, s.buildTelegramFailureEvent(keyword, event, err))
			continue
		}

		_ = s.alertEvents.MarkSentToTelegram(ctx, event.ID)
	}
}

func isActionableAlert(event domain.AlertEvent) bool {
	if event.Level != domain.AlertLevelAlert {
		return false
	}

	switch event.EventType {
	case domain.AlertEventTypeThresholdHit, domain.AlertEventTypeNewLowest, domain.AlertEventTypePriceDrop:
		return true
	default:
		return false
	}
}

func buildTelegramPayload(
	keyword domain.TrackedKeyword,
	snapshot domain.MarketSnapshot,
	grouped []domain.GroupedListing,
	event domain.AlertEvent,
) dto.TelegramAlertPayload {
	payload := dto.TelegramAlertPayload{
		TrackedKeywordID: keyword.ID,
		Keyword:          keyword.Keyword,
		EventType:        string(event.EventType),
		Signal:           string(snapshot.Signal),
		Message:          event.Message,
		BestPrice:        snapshot.MinPrice,
		ThresholdPrice:   keyword.ThresholdPrice,
		SnapshotAt:       snapshot.SnapshotAt,
	}

	if top := selectTopListing(grouped); top != nil {
		payload.TopListing = &dto.TelegramTopListing{
			RepresentativeTitle:  top.RepresentativeTitle,
			RepresentativeSeller: top.RepresentativeSeller,
			BestPrice:            top.BestPrice,
			SampleURL:            top.SampleURL,
		}
	}

	return payload
}

func selectTopListing(grouped []domain.GroupedListing) *domain.GroupedListing {
	if len(grouped) == 0 {
		return nil
	}

	best := grouped[0]
	for _, listing := range grouped[1:] {
		if listing.BestPrice < best.BestPrice {
			best = listing
			continue
		}
		if listing.BestPrice == best.BestPrice && listing.ListingCount > best.ListingCount {
			best = listing
		}
	}

	return &best
}

func (s *Service) buildTelegramFailureEvent(keyword domain.TrackedKeyword, sourceEvent domain.AlertEvent, sendErr error) domain.AlertEvent {
	payloadJSON, _ := json.Marshal(map[string]string{
		"source_event_id":   stringValue(sourceEvent.ID),
		"source_event_type": string(sourceEvent.EventType),
		"error":             sendErr.Error(),
	})
	payload := string(payloadJSON)

	return domain.AlertEvent{
		ID:               s.idGenerator.Next(),
		TrackedKeywordID: keyword.ID,
		ScanJobID:        sourceEvent.ScanJobID,
		Level:            domain.AlertLevelWarn,
		EventType:        domain.AlertEventTypeTelegramFail,
		Message:          fmt.Sprintf("Telegram delivery failed for %s", sourceEvent.EventType),
		PayloadJSON:      &payload,
		SentToTelegram:   false,
		CreatedAt:        s.clock.Now(),
	}
}

func stringValue(value string) string {
	return value
}
