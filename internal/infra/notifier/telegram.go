package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pricealert/pricealert/internal/config"
	"github.com/pricealert/pricealert/internal/dto"
)

type Telegram struct {
	client     *http.Client
	botToken   string
	chatID     string
	apiBaseURL string
}

type sendMessageRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
}

type sendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func NewTelegram(cfg config.TelegramConfig) *Telegram {
	return &Telegram{
		client:     &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
		botToken:   cfg.BotToken,
		chatID:     cfg.ChatID,
		apiBaseURL: strings.TrimRight(cfg.APIBaseURL, "/"),
	}
}

func NewTelegramWithClient(cfg config.TelegramConfig, client *http.Client) *Telegram {
	telegram := NewTelegram(cfg)
	if client != nil {
		telegram.client = client
	}
	return telegram
}

func (t *Telegram) SendAlert(ctx context.Context, payload dto.TelegramAlertPayload) error {
	body, err := json.Marshal(sendMessageRequest{
		ChatID:                t.chatID,
		Text:                  formatTelegramMessage(payload),
		DisableWebPagePreview: true,
	})
	if err != nil {
		return fmt.Errorf("marshal telegram request: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", t.apiBaseURL, t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram request: %w", err)
	}
	defer resp.Body.Close()

	var envelope sendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode telegram response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !envelope.OK {
		if envelope.Description == "" {
			envelope.Description = resp.Status
		}
		return fmt.Errorf("telegram send failed: %s", envelope.Description)
	}

	return nil
}

func formatTelegramMessage(payload dto.TelegramAlertPayload) string {
	lines := []string{
		fmt.Sprintf("PriceAlert: %s", payload.Keyword),
		fmt.Sprintf("Alert: %s", payload.EventType),
		fmt.Sprintf("Signal: %s", payload.Signal),
		fmt.Sprintf("Message: %s", payload.Message),
	}

	if payload.BestPrice != nil {
		lines = append(lines, fmt.Sprintf("Current Price: %d", *payload.BestPrice))
	}
	if payload.ThresholdPrice != nil {
		lines = append(lines, fmt.Sprintf("Threshold: %d", *payload.ThresholdPrice))
	}
	if payload.TopListing != nil {
		lines = append(lines, fmt.Sprintf("Top Listing: %s", payload.TopListing.RepresentativeTitle))
		lines = append(lines, fmt.Sprintf("Seller: %s", payload.TopListing.RepresentativeSeller))
		if payload.TopListing.SampleURL != "" {
			lines = append(lines, fmt.Sprintf("Link: %s", payload.TopListing.SampleURL))
		}
	}

	return strings.Join(lines, "\n")
}

type Noop struct{}

func NewNoop() Noop {
	return Noop{}
}

func (Noop) SendAlert(context.Context, dto.TelegramAlertPayload) error {
	return nil
}
