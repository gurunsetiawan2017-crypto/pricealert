package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pricealert/pricealert/internal/config"
	"github.com/pricealert/pricealert/internal/dto"
)

func TestTelegramSendAlertPostsFormattedMessage(t *testing.T) {
	var gotPath string
	var gotRequest sendMessageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(sendMessageResponse{OK: true})
	}))
	defer server.Close()

	client := NewTelegramWithClient(config.TelegramConfig{
		BotToken:       "token123",
		ChatID:         "chat456",
		APIBaseURL:     server.URL,
		TimeoutSeconds: 5,
	}, server.Client())

	price := int64(23800)
	err := client.SendAlert(context.Background(), dto.TelegramAlertPayload{
		Keyword:        "minyak goreng 2L",
		EventType:      "threshold_hit",
		Signal:         "BUY_NOW",
		Message:        "minyak goreng 2L hit threshold",
		BestPrice:      &price,
		ThresholdPrice: &price,
		TopListing: &dto.TelegramTopListing{
			RepresentativeTitle:  "Minyak Goreng 2L Promo",
			RepresentativeSeller: "Seller A",
			BestPrice:            23800,
			SampleURL:            "https://example.com/item/1",
		},
	})
	if err != nil {
		t.Fatalf("SendAlert() error = %v", err)
	}

	if gotPath != "/bottoken123/sendMessage" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotRequest.ChatID != "chat456" {
		t.Fatalf("chat id = %q", gotRequest.ChatID)
	}
	if !strings.Contains(gotRequest.Text, "PriceAlert: minyak goreng 2L") {
		t.Fatalf("text = %q", gotRequest.Text)
	}
	if !strings.Contains(gotRequest.Text, "Link: https://example.com/item/1") {
		t.Fatalf("text = %q", gotRequest.Text)
	}
}

func TestTelegramSendAlertReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(sendMessageResponse{OK: false, Description: "bad request"})
	}))
	defer server.Close()

	client := NewTelegramWithClient(config.TelegramConfig{
		BotToken:       "token123",
		ChatID:         "chat456",
		APIBaseURL:     server.URL,
		TimeoutSeconds: 5,
	}, server.Client())

	err := client.SendAlert(context.Background(), dto.TelegramAlertPayload{
		Keyword:   "minyak goreng 2L",
		EventType: "threshold_hit",
		Signal:    "BUY_NOW",
		Message:   "threshold hit",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
